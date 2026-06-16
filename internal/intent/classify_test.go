package intent

import "testing"

func TestClassifyVerifyFailure(t *testing.T) {
	got := Classify("CI is red and tests fail on verify step")
	if got.Class != ClassVerifyFailure {
		t.Fatalf("class = %q, want %q", got.Class, ClassVerifyFailure)
	}
}

func TestClassifyExplore(t *testing.T) {
	got := Classify("快速探索此项目：找出主入口、核心模块和测试布局")
	if got.Class != ClassExplore {
		t.Fatalf("class = %q, want %q", got.Class, ClassExplore)
	}
}

func TestClassifyWrite(t *testing.T) {
	got := Classify("/write polish this paragraph")
	if got.Class != ClassWrite {
		t.Fatalf("class = %q, want %q", got.Class, ClassWrite)
	}
}

func TestClassifyQA(t *testing.T) {
	got := Classify("解释一下这个函数是干什么的")
	if got.Class != ClassQA {
		t.Fatalf("class = %q, want %q", got.Class, ClassQA)
	}
}

func TestClassifyRefactor(t *testing.T) {
	got := Classify("refactor the frontend routing layer across multiple files")
	if got.Class != ClassRefactor {
		t.Fatalf("class = %q, want %q", got.Class, ClassRefactor)
	}
}
