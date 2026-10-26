package main

import (
	"reflect"
	"testing"
)

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

func TestReplayBrokenTail(t *testing.T) {
	s := &store{data: map[string]string{}}
	err := s.replay("testdata/broken_tail.log")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	want := map[string]string{"a": "1", "b": "2"}
	if !reflect.DeepEqual(s.data, want) {
		t.Fatalf("got %v, want %v", s.data, want)
	}
}

func TestReplayBrokenMiddle(t *testing.T) {
	s := &store{data: map[string]string{}}
	err := s.replay("testdata/broken_middle.log")
	if err == nil {
		t.Fatalf("want error for broken middle line")
	}
}
