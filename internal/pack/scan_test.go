package pack

import "testing"

func TestParseTarStem(t *testing.T) {
	cases := []struct {
		stem, parent, name, ver string
	}{
		{"mysql5.7.44", "mysql", "mysql", "5.7.44"},
		{"redis.6.0", "redis", "redis", "6.0"},
		{"nacos-server.v2.2.3-slim", "nacos", "nacos-server", "v2.2.3-slim"},
		{"kafka.2.12-2.4.1", "kafka", "kafka", "2.12-2.4.1"},
		{"zookeeper.3.4.13", "kafka", "zookeeper", "3.4.13"},
		{"mongodb_v4-4-11", "mongodb", "mongodb", "v4-4-11"},
		{"minio.20210406", "minio", "minio", "20210406"},
		{"emqx.4.3.12", "emqx", "emqx", "4.3.12"},
		{"nginx.1.24.0", "nginx", "nginx", "1.24.0"},
		{"pgsql.14.5", "pgsql", "pgsql", "14.5"},
		{"postgis.14.15", "postgis", "postgis", "14.15"},
		{"influxdb.2.4", "influxdb", "influxdb", "2.4"},
		{"kafdrop", "kafka", "kafdrop", ""},
		{"water-job-biz.v4.6.2", "waterjob", "water-job-biz", "v4.6.2"},
		{"waterwork-center-4.0.2", "images", "waterwork-center", "4.0.2"},
	}
	for _, c := range cases {
		n, v := parseTarStem(c.stem, c.parent)
		if n != c.name || v != c.ver {
			t.Fatalf("%s (parent=%s): got (%s,%s) want (%s,%s)", c.stem, c.parent, n, v, c.name, c.ver)
		}
	}
}

func TestNormalizeServiceName(t *testing.T) {
	if normalizeServiceName("nacos-server") != "nacos" {
		t.Fatal("nacos-server")
	}
	if normalizeServiceName("water-job-biz") != "waterjob" {
		t.Fatal("water-job-biz")
	}
}
