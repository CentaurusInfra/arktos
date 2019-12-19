package bcast

/*
   bcast package for Go. Broadcasting on a set of channels.

   Copyright © 2013 Alexander I.Grafov <grafov@gmail.com>.
   All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the LICENSE file.
*/

import (
	"gopkg.in/fatih/set.v0"
	"testing"
	"time"
)

// Create new broadcast group.
// Join two members.
func TestNewGroupAndJoin(t *testing.T) {
	group := NewGroup()
	member1 := group.Join()
	member2 := group.Join()
	if member1.group != member2.group {
		t.Fatal("group for these members must be same")
	}
}

// Create new broadcast group.
// Join two members.
// Unjoin first member.
func TestUnjoin(t *testing.T) {
	group := NewGroup()
	member1 := group.Join()
	member2 := group.Join()
	if len(group.members) != 2 {
		t.Fatal("incorrect length of `out` slice")
	}
	go group.Broadcast(2 * time.Second)

	member1.Close()
	if len(group.members) > 1 || group.members[0] != member2 {
		t.Fatal("unjoin member does not work")
	}
}

// Create new broadcast group.
// Join 12 members.
// Make group broadcast to all group members.
func TestMemberBroadcast(t *testing.T) {
	group := NewGroup()
	var channels []chan bool
	max := 12
	broadcaster := 3

	for i := 0; i <= max; i++ {
		c := make(chan bool)
		channels = append(channels, c)
	}

	for i, c := range channels {
		m := group.Join()
		go func(i int, group *Group, channel chan bool, member *Member) {
			if i == broadcaster {
				m.Send(i)
			} else {
				val := m.Recv()
				if val != broadcaster {
					t.Fatal("incorrect message received")
				}
			}
			channel <- true
			val := m.Recv()
			if val != "done" {
				t.Fatal("incorrect message received")
			}
			channel <- true
		}(i, group, c, m)
	}

	go group.Broadcast(0)
	for _, channel := range channels {
		<-channel
	}
	group.Send("done")
	for _, channel := range channels {
		<-channel
	}
}

// Create new broadcast group.
// Join 12 members.
// Make group broadcast to all group members.
func TestGroupBroadcast(t *testing.T) {
	group := NewGroup()
	var channels []chan bool
	max := 12

	for i := 0; i <= max; i++ {
		c := make(chan bool)
		channels = append(channels, c)
	}

	for i, c := range channels {
		m := group.Join()
		go func(i int, group *Group, channel chan bool, member *Member) {
			val := m.Recv()
			if val != "group message" {
				t.Fatal("incorrect message received")
			}
			channel <- true
		}(i, group, c, m)
	}

	go group.Broadcast(0)
	group.Send("group message")
	for _, channel := range channels {
		<-channel
	}
}

// Create new broadcast group.
// Join 128 members.
// Make group broadcast to all members.
func TestBroadcastOnLargeNumberOfMembers(t *testing.T) {
	const max = 128
	var channels []chan *set.Set
	var members []*Member
	expected := set.New()

	group := NewGroup()
	for i := 0; i <= max; i++ {
		expected.Add(i)
		m := group.Join()
		members = append(members, m)
	}
	for i, member := range members {
		c := make(chan *set.Set)
		channels = append(channels, c)
		go func(i int, group *Group, channel chan *set.Set, m *Member) {
			m.Send(i)
			encountered := set.New()
			encountered.Add(i) // The message sent by this member wont be received
			for {
				newValue := m.Recv()
				if encountered.Has(newValue) {
					t.Fatal("Received duplicate value")
				}
				encountered.Add(newValue)
				if encountered.IsEqual(expected) {
					break
				}
			}
			channel <- encountered
		}(i, group, c, member)
	}
	go group.Broadcast(0)
	for _, channel := range channels {
		<-channel
	}
}
