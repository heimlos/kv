package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// 测正常日志的重放：put a=1 → put b=2 → 覆盖 a=3 → del b，最后只剩 {"a":"3"}，且不报错
func TestReplayNormal(t *testing.T) {
	s := &store{data: map[string]string{}}
	err := s.replay("testdata/normal.log")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	want := map[string]string{"a": "3"}
	if !reflect.DeepEqual(s.data, want) {
		t.Fatalf("got %v, want %v", s.data, want)
	}
}

// 测空文件：重放出空 map，不报错
func TestReplayEmpty(t *testing.T) {
	s := &store{data: map[string]string{}}
	err := s.replay("testdata/empty.log")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	want := map[string]string{}
	if !reflect.DeepEqual(s.data, want) {
		t.Fatalf("got %v, want %v", s.data, want)
	}
}

// 测尾部半截：半截行当"崩溃尾巴"忽略，不报错，半截之前的 a、b 都在
func TestReplayBrokenTail(t *testing.T) {
	// 把 testdata 里的文件复制到临时目录，免得真把测试数据截短
	src, _ := os.ReadFile("testdata/broken_tail.log")
	path := filepath.Join(t.TempDir(), "broken_tail.log")
	os.WriteFile(path, src, 0644)

	s := &store{data: map[string]string{}}
	err := s.replay(path)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	want := map[string]string{"a": "1", "b": "2"}

	got, err := os.Stat(path) // 看副本现在的字节数
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got.Size() >= int64(len(src)) { // 原来是 len(src)，截断后必须更小
		t.Fatalf("want truncated file, still %d bytes", got.Size())
	}

	if !reflect.DeepEqual(s.data, want) {
		t.Fatalf("got %v, want %v", s.data, want)
	}
}

// 测中间坏行：属于真损坏，必须返回 error（不能静默跳过）
func TestReplayBrokenMiddle(t *testing.T) {
	s := &store{data: map[string]string{}}
	err := s.replay("testdata/broken_middle.log")
	if err == nil {
		t.Fatalf("want error for broken middle line")
	}
}
