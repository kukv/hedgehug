package change

import (
	"fmt"
	"strings"
)

// BotLogin は、人でない実行者の行の by と、そのコミットの作者の名前(01「「あなた」の決まり方」)。
const BotLogin = "hedgehug[bot]"

// ActorKind は実行者の種類(保存形式「変更記録」の actor)。
type ActorKind string

const (
	ActorUser       ActorKind = "user"
	ActorAutomation ActorKind = "automation"
	ActorGitHub     ActorKind = "github"
	ActorSync       ActorKind = "sync"
	ActorNone       ActorKind = "none"
)

// Actor は 1 回の操作の実行者。Login は人のログイン名(自動化は、きっかけになった操作をした人)。
type Actor struct {
	Kind  ActorKind
	Login string
	Rule  ID // 自動化のルールの内部 ID
}

// User は、人が行った操作の実行者。
func User(login string) Actor { return Actor{Kind: ActorUser, Login: login} }

// By は、行の by に書く値。
func (a Actor) By() string {
	if a.Kind == ActorUser || a.Kind == ActorAutomation {
		return a.Login
	}
	return BotLogin
}

// Code は、行の actor に書く値。
func (a Actor) Code() string {
	if a.Kind == ActorAutomation {
		return "automation:" + string(a.Rule)
	}
	return string(a.Kind)
}

// ParseActor は、行の actor と by から実行者を戻す。
func ParseActor(code, by string) (Actor, error) {
	if rule, ok := strings.CutPrefix(code, "automation:"); ok {
		return Actor{Kind: ActorAutomation, Login: by, Rule: ID(rule)}, nil
	}
	switch k := ActorKind(code); k {
	case ActorUser:
		return User(by), nil
	case ActorGitHub, ActorSync, ActorNone:
		return Actor{Kind: k}, nil
	}
	return Actor{}, fmt.Errorf("unknown actor %q", code)
}
