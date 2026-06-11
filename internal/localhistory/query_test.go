package localhistory

import (
	"context"
	"testing"
)

func TestGetRecentChanges(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)

	_, _ = s.SaveVersion(ctx, "a.txt", []byte("a1"), false, "main", 0)
	b1, _ := s.SaveVersion(ctx, "b.txt", []byte("b1"), false, "main", 0)
	a2, _ := s.SaveVersion(ctx, "a.txt", []byte("a2"), false, "main", 0)

	// 给 b.txt 的 b1 版打个标签，验证 label_name 关联回填。
	if err := s.PutLabel(ctx, HistoryLabel{
		ID: "L", Name: "tag-b", LabelType: "manual", Source: "user",
		Timestamp: "2026-05-30T10:00:00Z", Branch: "main",
		FileSnapshots: []LabelFileSnapshot{{FilePath: "b.txt", VersionID: b1.ID}},
	}); err != nil {
		t.Fatalf("put label: %v", err)
	}

	rc, err := s.GetRecentChanges(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(rc) != 3 {
		t.Fatalf("recent changes = %d, want 3", len(rc))
	}
	// 最新在前：a.txt 的 a2 最后保存。
	if rc[0].FilePath != "a.txt" || rc[0].VersionID != a2.ID {
		t.Errorf("newest = %s/%s, want a.txt/%s", rc[0].FilePath, rc[0].VersionID, a2.ID)
	}
	// 标签名只在被引用的 b.txt/b1 上出现。
	for _, c := range rc {
		want := ""
		if c.FilePath == "b.txt" && c.VersionID == b1.ID {
			want = "tag-b"
		}
		if c.LabelName != want {
			t.Errorf("%s/%s labelName = %q, want %q", c.FilePath, c.VersionID, c.LabelName, want)
		}
	}
	// limit 生效。
	if one, _ := s.GetRecentChanges(ctx, 1); len(one) != 1 || one[0].VersionID != a2.ID {
		t.Errorf("limit 1 = %v, want [a2]", one)
	}
}

func TestListDeletedFiles(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	_, _ = s.SaveVersion(ctx, "a.txt", []byte("a1"), false, "main", 0)
	_, _ = s.SaveVersion(ctx, "a.txt", []byte("a1"), true, "main", 0) // a.txt 最新 = 删除
	_, _ = s.SaveVersion(ctx, "b.txt", []byte("b1"), false, "main", 0)

	del, err := s.ListDeletedFiles(ctx)
	if err != nil {
		t.Fatalf("ListDeletedFiles: %v", err)
	}
	if len(del) != 1 || del[0].FilePath != "a.txt" || !del[0].IsDeleted {
		t.Fatalf("deleted = %+v, want [a.txt deleted]", del)
	}

	// a.txt 又复活（最新非删除）→ 不再算已删除。
	_, _ = s.SaveVersion(ctx, "a.txt", []byte("a2"), false, "main", 0)
	if del, _ := s.ListDeletedFiles(ctx); len(del) != 0 {
		t.Fatalf("after revive deleted = %d, want 0", len(del))
	}
}
