package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"RJDJ/backend/internal/middleware"
	"RJDJ/backend/internal/model"
	"RJDJ/backend/internal/okx"
)

// ============= 合约账单缓存（入库去重，命中缓存不重复拉 OKX） =============
// 思路：账单只在时间上向后追加。缓存写入时按 (config_id, billId) 去重。
// 任务策略：
//   - 若该 config 库里“最旧账单仍晚于目标 cutoff”（即还没覆盖到目标天数）或库为空
//     → 从最新整段翻页写库，直到翻到 cutoff 之前。
//   - 否则（已覆盖到目标天数）→ 仅从最新增量探测：一旦翻到一条已在库的账单即停，
//     把“上次之后新增的账单”补进库（通常 1~2 页），然后直接读库聚合。

func billFromOKX(b *okx.Bill) model.OkxBill {
	toF := func(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }
	toI := func(s string) int64 { i, _ := strconv.ParseInt(s, 10, 64); return i }
	raw, _ := json.Marshal(b)
	return model.OkxBill{
		BillID:   b.BillID,
		InstType: b.InstType,
		InstID:   b.InstID,
		Ccy:      b.Ccy,
		Pnl:      toF(b.Pnl),
		Fee:      toF(b.Fee),
		BalChg:   toF(b.BalChg),
		Type:     b.Type,
		SubType:  b.SubType,
		Ts:       toI(b.Ts),
		Raw:      string(raw),
	}
}

// existingBillIDs 返回某 config 已缓存的全部 billId 集合（用于增量停判）。
func (h *OkxDataHandler) existingBillIDs(configID uint) map[string]bool {
	set := map[string]bool{}
	var ids []string
	h.db.Model(&model.OkxBill{}).Where("config_id = ?", configID).
		Pluck("bill_id", &ids)
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// loadCachedBills 读取某 config 在 cutoff(毫秒) 之后的合约账单（SWAP+FUTURES）。
func (h *OkxDataHandler) loadCachedBills(configID uint, cutoff int64) []model.OkxBill {
	var bills []model.OkxBill
	h.db.Where("config_id = ? AND inst_type IN ? AND ts >= ?",
		configID, []string{"SWAP", "FUTURES"}, cutoff).
		Order("ts asc").Find(&bills)
	return bills
}

// oldestCachedTS 某 config 已缓存账单的最早时间(毫秒)；空则返回 0。
func (h *OkxDataHandler) oldestCachedTS(configID uint) int64 {
	var mn struct{ Min int64 }
	h.db.Model(&model.OkxBill{}).
		Where("config_id = ? AND inst_type IN ?", configID, []string{"SWAP", "FUTURES"}).
		Select("MIN(ts) as min").Scan(&mn)
	return mn.Min
}

// ============= 异步任务表 =============

type pnlTask struct {
	ID        string
	UserID    uint
	ConfigID  uint
	Days      int
	Asset     string // 标的币筛选（如 BTC/ETH），空=全部
	MaxPages  int            // 每类型最大翻页预算（用于估算最坏时长）
	PageMs    int            // 每页限速间隔(ms)
	startedAt time.Time
	mu        sync.Mutex
	Pages     int  // 已翻页数
	Covered   int  // 缓存已覆盖天数（>=请求天数即认为已覆盖）
	NewSaved  int  // 本次新增写入的账单数
	Done      bool
	Err       string
	ElapsedMs int64
	Summary   *okx.PnlSummary
}

func (t *pnlTask) snapshot() map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()
	var summary *okx.PnlSummary
	if t.Summary != nil {
		s := *t.Summary
		summary = &s
	}
	remain := 0
	if !t.Done {
		// 最坏还需翻页数 = 预算上限 - 已翻；估算剩余秒
		remPages := (t.MaxPages*2 - t.Pages)
		if remPages < 0 {
			remPages = 0
		}
		remain = remPages * t.PageMs / 1000
		if remain < 1 {
			remain = 1
		}
	}
	return map[string]any{
		"id":            t.ID,
		"done":          t.Done,
		"error":         t.Err,
		"pages":         t.Pages,
		"covered_days":  t.Covered,
		"new_saved":     t.NewSaved,
		"elapsed_ms":    time.Since(t.startedAt).Milliseconds(),
		"est_remain_sec": remain,
		"days":          t.Days,
		"asset":         t.Asset,
		"summary":       summary,
	}
}

var (
	pnlTaskMu sync.Mutex
	pnlTasks  = map[string]*pnlTask{}
)

func newTaskID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// StartPnlFetch 启动一个异步账单拉取任务：POST /api/okx/pnl/daily/start
// 参数: days(30/90), asset(可选标的币如 BTC/ETH), config_id(可选)
func (h *OkxDataHandler) StartPnlFetch(c *gin.Context) {
	uid := middleware.CurrentUser(c)
	cfg, cred, err := h.resolveConfig(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	days := atoiDefault(c.Query("days"), 90)
	if days <= 0 {
		days = 90
	}
	if days > 180 {
		days = 180
	}
	asset := strings.ToUpper(strings.TrimSpace(c.Query("asset")))
	client := h.buildOKXClient(cred)

	const pageInterval = 350 * time.Millisecond
	const maxPagesPerType = 60 // 每类型 ≤ 60 页(=6000条) 保护

	t := &pnlTask{
		ID:        newTaskID(),
		UserID:    uid,
		ConfigID:  cfg.ID,
		Days:      days,
		Asset:     asset,
		MaxPages:  maxPagesPerType,
		PageMs:    int(pageInterval / time.Millisecond),
		startedAt: time.Now(),
	}
	pnlTaskMu.Lock()
	pnlTasks[t.ID] = t
	pnlTaskMu.Unlock()

	// 最坏估算（预算上限页 * 限速 + 聚合余量），用于前端第一时间显示预计区间
	worst := (maxPagesPerType*2)*int(pageInterval/time.Millisecond)/1000 + 3

	go h.runPnlTask(client, t, pageInterval, maxPagesPerType)

	c.JSON(http.StatusOK, gin.H{
		"task_id":         t.ID,
		"days":            days,
		"asset":           asset,
		"page_interval_ms": int(pageInterval / time.Millisecond),
		"est_seconds":     worst,
	})
}

func (h *OkxDataHandler) runPnlTask(client *okx.Client, t *pnlTask, pageInterval time.Duration, maxPagesPerType int) {
	cutoff := time.Now().AddDate(0, 0, -t.Days).UnixMilli()
	existing := h.existingBillIDs(t.ConfigID)
	// 库为空 或 库内最旧账单仍晚于目标 cutoff → 需整段补齐（可能较多页）
	// 否则 → 仅增量探测最新新增（通常 1~2 页）即停
	needCoverFull := len(existing) == 0 || h.oldestCachedTS(t.ConfigID) > cutoff

	opts := okx.FetchOpts{
		Days:            t.Days,
		MaxPagesPerType: maxPagesPerType,
		PageInterval:    pageInterval,
	}
	fetchErr := client.FetchContractBillsWithProgress(opts, func(ev okx.BillPageEvent) (bool, error) {
		t.mu.Lock()
		t.Pages = ev.TotalPages
		t.mu.Unlock()
		// 组装待新增账单
		var toSave []model.OkxBill
		stop := false
		for _, b := range ev.Bills {
			if _, ok := existing[b.BillID]; ok {
				// 命中缓存：只有“已覆盖足够天数”(needCoverFull=false) 时才允许提前停，
				// 整段同步(needCoverFull=true)时命中说明早已有，但更早未必覆盖满，继续翻由 cutoff 结束。
				if !needCoverFull {
					stop = true
					break
				}
				continue
			}
			mb := billFromOKX(&b)
			if mb.BillID == "" {
				continue
			}
			mb.UserID = t.UserID
			mb.ConfigID = t.ConfigID
			toSave = append(toSave, mb)
			existing[b.BillID] = true
		}
		if len(toSave) > 0 {
			if err := h.db.CreateInBatches(toSave, 200).Error; err != nil {
				return false, err
			}
			t.mu.Lock()
			t.NewSaved += len(toSave)
			t.mu.Unlock()
		}
		if stop {
			return true, nil
		}
		return false, nil
	})
	if fetchErr != nil {
		t.mu.Lock()
		t.Done = true
		t.Err = fetchErr.Error()
		t.mu.Unlock()
		return
	}

	// 从库聚合
	cached := h.loadCachedBills(t.ConfigID, cutoff)
	var summary okx.PnlSummary
	if t.Asset != "" {
		summary = okx.AggregatePnl(okx.FilterBillsByAsset(okxBills(cached), t.Asset))
	} else {
		summary = okx.AggregatePnl(okxBills(cached))
	}
	// 已覆盖天数（用于进度显示）：库内最早 距今
	covered := 0
	if len(cached) > 0 {
		oldest := cached[0].Ts
		covered = int((time.Now().UnixMilli() - oldest) / int64(86400000))
	}

	t.mu.Lock()
	t.Done = true
	t.Summary = &summary
	t.Covered = covered
	t.mu.Unlock()
}

// PnlFetchStatus 轮询异步任务状态：GET /api/okx/pnl/daily/status?task_id=
func (h *OkxDataHandler) PnlFetchStatus(c *gin.Context) {
	id := c.Query("task_id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task_id 必填"})
		return
	}
	pnlTaskMu.Lock()
	t, ok := pnlTasks[id]
	pnlTaskMu.Unlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在或已过期"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": t.snapshot()})
}

// okxBills 把缓存 model 转成聚合所需的 okx.Bill。
func okxBills(bills []model.OkxBill) []okx.Bill {
	out := make([]okx.Bill, 0, len(bills))
	for _, mb := range bills {
		out = append(out, okx.Bill{
			BillID:   mb.BillID,
			Ts:       strconv.FormatInt(mb.Ts, 10),
			InstType: mb.InstType,
			InstID:   mb.InstID,
			Ccy:      mb.Ccy,
			Pnl:      strconv.FormatFloat(mb.Pnl, 'f', -1, 64),
			Fee:      strconv.FormatFloat(mb.Fee, 'f', -1, 64),
			BalChg:   strconv.FormatFloat(mb.BalChg, 'f', -1, 64),
			Type:     mb.Type,
			SubType:  mb.SubType,
		})
	}
	// 按 ts 升序，保证 daily 序列稳定
	sort.Slice(out, func(i, j int) bool { return toI64(out[i].Ts) < toI64(out[j].Ts) })
	return out
}

func toI64(s string) int64 { i, _ := strconv.ParseInt(s, 10, 64); return i }

// StartPnlTaskJanitor 后台清理过期任务（防内存泄漏）。
func StartPnlTaskJanitor() {
	go func() {
		for {
			time.Sleep(30 * time.Minute)
			now := time.Now()
			pnlTaskMu.Lock()
			for id, t := range pnlTasks {
				if now.Sub(t.startedAt) > 30*time.Minute {
					delete(pnlTasks, id)
				}
			}
			pnlTaskMu.Unlock()
		}
	}()
}
