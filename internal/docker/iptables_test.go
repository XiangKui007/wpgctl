package dockerx

import (
	"errors"
	"testing"
)

func TestIptablesBroken(t *testing.T) {
	err := errors.New(`docker compose up 失败: Creating network "kafka_default" with the default driver Failed to Setup IP tables: Unable to enable SKIP DNAT rule: (iptables failed: iptables --wait -t nat -I DOCKER -i br-ed33c7341da6 -j RETURN: iptables: No chain/target/match by that name. (exit status 1))`)
	if !IptablesBroken(err) {
		t.Fatal("expected match")
	}
	if IptablesBroken(errors.New("本地缺少镜像")) {
		t.Fatal("false positive")
	}
	if IptablesBroken(nil) {
		t.Fatal("nil")
	}
}
