//go:build !e2e

package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/member"
)

// 重なった Issue のリンクと、大文字小文字だけが違うメンバーの修復と、その通知を、本物の gitstore と repairing.Run と
// state.db の受信箱で、2 台のマシンの場面として確かめる(C5c2)。A のログイン名は taro、B は hanako。

// linkIssueAs は、login の操作として、アイテム id に Issue kukv/web#number をリンクする。
func linkIssueAs(t *testing.T, m machine, login string, id item.ID, number int) {
	t.Helper()
	ref := itemlink.Ref{Kind: itemlink.Issue, Repo: "kukv/web", Number: number}
	v := itemlink.Values{Title: "課題", State: itemlink.Open, Review: itemlink.NoReview}
	m.writeAs(t, login, "Issue をリンク", func(tx port.Tx) error {
		return tx.WriteItems().Append(id, itemlink.Set{}.Attach(ref, v)...)
	})
}

// issuesOf は、m のアイテム id にリンクされた Issue の番号(昇順)。
func issuesOf(t *testing.T, m machine, id item.ID) []int {
	t.Helper()
	var log []change.Record
	err := m.stores.Read(t.Context(), m.id, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var nums []int
	for _, l := range itemlink.FromLog(log).Links {
		nums = append(nums, l.Number)
	}
	slices.Sort(nums)
	return nums
}

// wantIssues は、ms のどのマシンでも、アイテム id にリンクされた Issue の番号が want なことを確かめる。
func wantIssues(t *testing.T, id item.ID, want []int, ms ...machine) {
	t.Helper()
	for _, m := range ms {
		if got := issuesOf(t, m, id); !slices.Equal(got, want) {
			t.Errorf("issues of %s on %s = %v, want %v", id, m.dir, got, want)
		}
	}
}

// wantCommit は、m のコミットのメッセージの 1 行目に summary があることを確かめる。
func wantCommit(t *testing.T, m machine, summary string) {
	t.Helper()
	repo, err := git.PlainOpen(m.dir)
	if err != nil {
		t.Fatal(err)
	}
	iter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer iter.Close()
	for c, err := iter.Next(); err == nil; c, err = iter.Next() {
		if first, _, _ := strings.Cut(c.Message, "\n"); first == summary {
			return
		}
	}
	t.Errorf("%s: no commit with the summary %q", m.dir, summary)
}

// A が HH-1 に Issue kukv/web#5 をリンクし、B が同時に HH-2 に同じ Issue をリンクする。A・B・A の順に同期すると、
// 2 台とも HH-1 だけがリンクを持つ。B(HH-2 にリンクした人)に 1 行、A には作らない。
func TestRepairIssueTwoItems(t *testing.T) {
	remote, a, b := noticePair(t)
	linkIssueAs(t, a, "taro", apptest.SampleEpic, 5)
	linkIssueAs(t, b, "hanako", apptest.SampleTask, 5)
	syncAll(t, remote, a, b)
	sameFiles(t, a, b)
	wantIssues(t, apptest.SampleEpic, []int{5}, a, b)
	wantIssues(t, apptest.SampleTask, nil, a, b)
	wantRepair(t, b, "Issue kukv/web#5 は HH-1 にリンクされているため、HH-2 からリンクを外しました")
	wantNoRepair(t, a)
}

// A が HH-1 に #5、B が HH-1 に #9 をリンクする。A・B・A の順に同期すると、2 台とも HH-1 は #5 だけ。
// B(#9 をリンクした人)に 1 行、A には作らない。
func TestRepairIssueOneItem(t *testing.T) {
	remote, a, b := noticePair(t)
	linkIssueAs(t, a, "taro", apptest.SampleEpic, 5)
	linkIssueAs(t, b, "hanako", apptest.SampleEpic, 9)
	syncAll(t, remote, a, b)
	sameFiles(t, a, b)
	wantIssues(t, apptest.SampleEpic, []int{5}, a, b)
	wantRepair(t, b, "HH-1 には Issue kukv/web#5 がリンクされているため、Issue kukv/web#9 のリンクを外しました")
	wantNoRepair(t, a)
}

// A が「Jiro」、B が「jiro」をメンバーに足す。A・B・A の順に同期すると、2 台ともメンバーは taro・hanako・Jiro の
// 3 人になる。どちらの受信箱にも同期の修復は出さず、B の修復の保存の件名は「メンバー Jiro の重複を統合」。
func TestRepairMembers(t *testing.T) {
	remote, a, b := noticePair(t)
	a.writeAs(t, "taro", "メンバーを追加", func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("Jiro")) })
	b.writeAs(t, "hanako", "メンバーを追加", func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("jiro")) })
	syncAll(t, remote, a, b)
	sameFiles(t, a, b)
	for _, m := range []machine{a, b} {
		var logins []string
		err := m.stores.Read(t.Context(), m.id, func(r port.Reader) error {
			ms, err := r.Members().List()
			for _, x := range ms {
				logins = append(logins, x.Login)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(logins)
		if want := []string{"Jiro", "hanako", "taro"}; !slices.Equal(logins, want) {
			t.Errorf("members on %s = %v, want %v", m.dir, logins, want)
		}
	}
	wantNoRepair(t, a)
	wantNoRepair(t, b)
	wantCommit(t, b, "メンバー Jiro の重複を統合")
}
