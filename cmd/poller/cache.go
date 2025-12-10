package main

import lru "github.com/hashicorp/golang-lru/v2"

var killmailCache, _ = lru.New[int32, bool](1024)

func isKillmailCached(killmailID int32) bool {
	ok, _ := killmailCache.ContainsOrAdd(killmailID, true)
	return ok
}
