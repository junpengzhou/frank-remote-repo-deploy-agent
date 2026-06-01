package maven

import "testing"

func TestBuildInstallCommandIncludesModuleSettingsLocalRepoAndJavaHome(t *testing.T) {
	cmd := BuildInstallCommand("/workspace", "example-frank", true, Options{
		Executable: "mvn",
		Settings:   "/opt/settings.xml",
		LocalRepo:  "/cache/.m2",
		ExtraArgs:  []string{"-DskipTests"},
		JavaHome:   "/usr/local/jdk-21",
	})
	wantArgs := []string{"clean", "install", "-pl", "example-frank", "-am", "-s", "/opt/settings.xml", "-Dmaven.repo.local=/cache/.m2", "-DskipTests"}
	if !equal(cmd.Args, wantArgs) {
		t.Fatalf("args mismatch\nwant %#v\n got %#v", wantArgs, cmd.Args)
	}
	if cmd.Env["JAVA_HOME"] != "/usr/local/jdk-21" {
		t.Fatalf("JAVA_HOME missing: %#v", cmd.Env)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
