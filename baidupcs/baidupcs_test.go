package baidupcs

import (
	"sync"
	"testing"
)

// 上传任务使用 CopyPCS() 的副本执行, 副本必须保留 pcs_addr_list,
// 否则轮询会静默回退到动态服务器逻辑
func TestCopyPCSPreservesAddrList(t *testing.T) {
	pcs := NewPCS(266719, "")
	pcs.SetPCSAddrList("c.pcs.baidu.com,c2.pcs.baidu.com,d.pcs.baidu.com")

	cp := pcs.CopyPCS()
	if len(cp.pcsAddrList) != 3 {
		t.Fatalf("副本丢失 pcsAddrList, 期望长度 3, 实际 %d", len(cp.pcsAddrList))
	}

	// 副本从复制的索引处继续轮询: 依次取 c, c2, d
	want := []string{"c.pcs.baidu.com", "c2.pcs.baidu.com", "d.pcs.baidu.com"}
	for _, w := range want {
		if got := cp.GetNextPCSHostFromList(); got != w {
			t.Fatalf("轮询取址错误: 期望 %s, 实际 %s", w, got)
		}
	}

	// 原实例的游标不应被副本扰动
	if got := pcs.GetNextPCSHostFromList(); got != want[0] {
		t.Fatalf("原实例游标被副本扰动: 期望 %s, 实际 %s", want[0], got)
	}
}

func TestSetPCSAddrListInvalid(t *testing.T) {
	pcs := NewPCS(266719, "")
	pcs.SetPCSAddrList("  c.pcs.baidu.com , d.pcs.baidu.com , , ")
	if len(pcs.pcsAddrList) != 2 {
		t.Fatalf("地址清洗错误: 期望 2 个有效地址, 实际 %d: %v", len(pcs.pcsAddrList), pcs.pcsAddrList)
	}
	pcs.SetPCSAddrList("")
	if pcs.pcsAddrList != nil || pcs.GetNextPCSHostFromList() != "" {
		t.Fatal("清空列表失败")
	}
}

// 上传期间后台 goroutine 会切换 pcsAddr, 其他分片 worker 并发读取它构造 URL,
// 必须无数据竞争 (go test -race 下验证)
func TestPCSAddrConcurrentAccess(t *testing.T) {
	pcs := NewPCS(266719, "")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			pcs.SetPCSAddr("d.pcs.baidu.com")
		}()
		go func() {
			defer wg.Done()
			_ = pcs.URL().Host
			_ = pcs.GetPCSAddr()
		}()
	}
	wg.Wait()
}
