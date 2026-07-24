package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServiceCatalogAndResolveByType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "services.cnf")
	contents := "[mysql_prod]\ntype=mysql\nhost=mysql.internal\nport=3306\nuser=backup\npassword=secret\n\n[redis_prod]\ntype=redis\nhost=redis.internal\nport=6379\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, err := LoadServiceCatalog(path)
	if err != nil {
		t.Fatalf("LoadServiceCatalog() error = %v", err)
	}
	profile, err := catalog.Resolve("mysql_prod", "mysql")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if profile.Host != "mysql.internal" || profile.Password != "secret" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	if _, err := catalog.Resolve("mysql_prod", "redis"); err == nil {
		t.Fatal("expected type mismatch")
	}
}

func TestLoadServiceCatalogRejectsInsecurePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "services.cnf")
	if err := os.WriteFile(path, []byte("[redis_prod]\ntype=redis\nhost=localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadServiceCatalog(path); err == nil {
		t.Fatal("expected insecure permissions error")
	}
}
