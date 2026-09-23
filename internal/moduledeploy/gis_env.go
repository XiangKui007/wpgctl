package moduledeploy

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wpg/wpgctl/internal/config"
)

// GIS 交付包默认库账号：与业务 PgSQL（wpg / 5433）不是同一套。
// PostGIS 走 5432、账号 postgres；MongoDB 走 27017、账号 gis。
const (
	gisPgSQLUser   = "postgres"
	gisPgSQLPass   = "GIS@city0418&wg"
	gisPgSQLPort   = 5432
	gisCenterDB    = "gis_center"
	gisDefaultDB   = "gis_default"
	gisMongoUser   = "gis"
	gisMongoPass   = "wpg87(GISklhjaal"
	gisMongoPort   = 27017
	gisMongoDBName = "gis"
)

// gisServiceKind 识别 GIS 子服务目录：giscenter → center，gisdefault → default。
func gisServiceKind(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, "_", "-")
	switch {
	case n == "giscenter" || n == "gis-center" || strings.HasPrefix(n, "giscenter-") || strings.HasPrefix(n, "gis-center-"):
		return "center"
	case n == "gisdefault" || n == "gis-default" || strings.HasPrefix(n, "gisdefault-") || strings.HasPrefix(n, "gis-default-"):
		return "default"
	default:
		return ""
	}
}

// gisEnvFlavor 根据 .env / compose 路径判断是 giscenter 还是 gisdefault；非 GIS 目录返回空。
func gisEnvFlavor(path string) string {
	p := strings.ToLower(filepath.ToSlash(path))
	parts := strings.Split(p, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if k := gisServiceKind(parts[i]); k != "" {
			return k
		}
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "gis" {
			return "center"
		}
	}
	return ""
}

// putGISEnvKeys 把 GIS 的 WPG_PGSQL_* / WPG_MONGODB_* 写入替换表。
// Host 来自节点上的 postgis / mongodb；库名：giscenter→gis_center，gisdefault→gis_default。
func putGISEnvKeys(repl map[string]string, site *config.SiteConfig, envPath string) {
	if repl == nil || site == nil {
		return
	}
	flavor := gisEnvFlavor(envPath)
	if flavor == "" {
		return
	}
	pgHost := hostOfService(site, "postgis", []string{"database"})
	if pgHost == "" {
		pgHost = hostOfService(site, "pgsql", []string{"database"})
	}
	if pgHost == "" {
		pgHost = strings.TrimSpace(site.Middleware.PgSQL.Host)
	}
	mongoHost := hostOfService(site, "mongodb", []string{"database"})
	if mongoHost == "" {
		mongoHost = pgHost
	}

	putKeys(repl, pgHost, "WPG_PGSQL_HOST", "WPG_PGSQL_ADDR", "WPG_PGSQL_IP", "WPG_PGSQL_HOSTNAME")
	putKeys(repl, strconv.Itoa(gisPgSQLPort), "WPG_PGSQL_PORT")
	putKeys(repl, gisPgSQLUser, "WPG_PGSQL_USER", "WPG_PGSQL_USERNAME")
	putKeys(repl, gisPgSQLPass, "WPG_PGSQL_PSWD", "WPG_PGSQL_PASSWORD", "WPG_PGSQL_PASSWD")
	dbName := gisCenterDB
	if flavor == "default" {
		dbName = gisDefaultDB
	}
	putKeys(repl, dbName, "WPG_PGSQL_DBNAME", "WPG_PGSQL_DATABASE", "WPG_PGSQL_DB")

	putKeys(repl, mongoHost, "WPG_MONGODB_HOST", "WPG_MONGODB_ADDR", "WPG_MONGODB_IP", "WPG_MONGODB_HOSTNAME")
	putKeys(repl, strconv.Itoa(gisMongoPort), "WPG_MONGODB_PORT")
	putKeys(repl, gisMongoUser, "WPG_MONGODB_USER", "WPG_MONGODB_USERNAME")
	putKeys(repl, gisMongoPass, "WPG_MONGODB_PSWD", "WPG_MONGODB_PASSWORD", "WPG_MONGODB_PASSWD")
	putKeys(repl, gisMongoDBName, "WPG_MONGODB_DBNAME", "WPG_MONGODB_DATABASE", "WPG_MONGODB_DB")
}
