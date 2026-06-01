package mcpe

import "testing"

func TestValidateRelativeNameRejectsTraversal(t *testing.T) {
	for _, bad := range []string{"..", ".", "../etc", "foo/bar", "foo\\bar", ""} {
		if err := validateRelativeName(bad); err == nil {
			t.Errorf("validateRelativeName(%q) should error", bad)
		}
	}
}

func TestValidateRelativeNameAcceptsValid(t *testing.T) {
	for _, ok := range []string{"183b0dc7-ae4a-48fb-800d-9d68f7162e7e", "my-world_01"} {
		if err := validateRelativeName(ok); err != nil {
			t.Errorf("validateRelativeName(%q) should be ok, got %v", ok, err)
		}
	}
}

func TestValidateCommandLinePathRejectsShellMetacharacters(t *testing.T) {
	bads := []string{
		`C:\hack"&calc&".exe`,
		"C:\\path\nwith\nnewline",
		`C:\foo|bar`,
		`C:\foo&bar`,
		"C:\\foo<bar",
		"C:\\foo%PATH%bar",
		"",
	}
	for _, bad := range bads {
		if err := validateCommandLinePath(bad); err == nil {
			t.Errorf("validateCommandLinePath(%q) should error", bad)
		}
	}
}

func TestValidateCommandLinePathAcceptsNormalWindows(t *testing.T) {
	oks := []string{
		`C:\Users\foo\MCStudioDownload\game\MinecraftPE_Netease\3.7.0.222545\Minecraft.Windows.exe`,
		"J:/Program/PE/MyMod",
		"C:\\空格 也允许\\foo",
	}
	for _, ok := range oks {
		if err := validateCommandLinePath(ok); err != nil {
			t.Errorf("validateCommandLinePath(%q) should be ok, got %v", ok, err)
		}
	}
}
