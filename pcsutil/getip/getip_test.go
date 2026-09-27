package getip

import (
	"os"
	"testing"
)

func TestGetIP(t *testing.T) {
	if os.Getenv("TEST_NETWORK") == "" {
		t.Skip("需要访问外部网络服务, 设置 TEST_NETWORK=1 启用")
	}

	ipAddr, err := IPInfo(false)
	if err != nil {
		t.Errorf("err: %s\n", err)
		return
	}

	t.Logf("from ipify: %s\n", ipAddr)

	ipAddr, err = IPInfoFromNetease()
	if err != nil {
		t.Errorf("err: %s\n", err)
		return
	}

	t.Logf("from netease: %s\n", ipAddr)
}
