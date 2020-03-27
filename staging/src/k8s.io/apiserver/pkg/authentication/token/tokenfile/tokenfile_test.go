/*
Copyright 2014 The Kubernetes Authors.
Copyright 2020 Authors of Arktos - file modified.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tokenfile

import (
	"context"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"k8s.io/apiserver/pkg/authentication/user"
)

func TestTokenFile(t *testing.T) {
	auth, err := newWithContents(t, `
token1,user1,uid1
token2,user2,uid2
token3,user3,uid3,"group1,group2"
token4,user4,uid4,"group2"
token5,user5,uid5,group5
token6,user6,uid6,group5,otherdata
token7,user7,uid7,"group1,group2",otherdata
token11,user11,uid11,,tenant11
token12,user12,uid12,"group1",,tenant12
token13,user13,uid13,group1,,tenant13
token14,user14,uid14,"group1,group2",,tenant14
token15,user15,uid15,"group1,group2",otherdata,,tenant15
`)
	if err != nil {
		t.Fatalf("unable to read tokenfile: %v", err)
	}

	testCases := []struct {
		Token string
		User  *user.DefaultInfo
		Ok    bool
		Err   bool
	}{
		{
			Token: "token1",
			User:  &user.DefaultInfo{Name: "user1", UID: "uid1", Tenant: "system"},
			Ok:    true,
		},
		{
			Token: "token2",
			User:  &user.DefaultInfo{Name: "user2", UID: "uid2", Tenant: "system"},
			Ok:    true,
		},
		{
			Token: "token3",
			User:  &user.DefaultInfo{Name: "user3", UID: "uid3", Tenant: "system", Groups: []string{"group1", "group2"}},
			Ok:    true,
		},
		{
			Token: "token4",
			User:  &user.DefaultInfo{Name: "user4", UID: "uid4", Tenant: "system", Groups: []string{"group2"}},
			Ok:    true,
		},
		{
			Token: "token5",
			User:  &user.DefaultInfo{Name: "user5", UID: "uid5", Tenant: "system", Groups: []string{"group5"}},
			Ok:    true,
		},
		{
			Token: "token6",
			User:  &user.DefaultInfo{Name: "user6", UID: "uid6", Tenant: "system", Groups: []string{"group5"}},
			Ok:    true,
		},
		{
			Token: "token7",
			User:  &user.DefaultInfo{Name: "user7", UID: "uid7", Tenant: "system", Groups: []string{"group1", "group2"}},
			Ok:    true,
		},
		{
			Token: "token8",
		},
		{
			Token: "token11",
			User:  &user.DefaultInfo{Name: "user11", UID: "uid11", Tenant: "tenant11"},
			Ok:    true,
		},
		{
			Token: "token12",
			User:  &user.DefaultInfo{Name: "user12", UID: "uid12", Groups: []string{"group1"}, Tenant: "tenant12"},
			Ok:    true,
		},
		{
			Token: "token13",
			User:  &user.DefaultInfo{Name: "user13", UID: "uid13", Groups: []string{"group1"}, Tenant: "tenant13"},
			Ok:    true,
		},
		{
			Token: "token14",
			User:  &user.DefaultInfo{Name: "user14", UID: "uid14", Groups: []string{"group1", "group2"}, Tenant: "tenant14"},
			Ok:    true,
		},
		{
			Token: "token15",
			User:  &user.DefaultInfo{Name: "user15", UID: "uid15", Groups: []string{"group1", "group2"}, Tenant: "tenant15"},
			Ok:    true,
		},
	}
	for i, testCase := range testCases {
		resp, ok, err := auth.AuthenticateToken(context.Background(), testCase.Token)
		if testCase.User == nil {
			if resp != nil {
				t.Errorf("%d: unexpected non-nil user %#v", i, resp.User)
			}
		} else if !reflect.DeepEqual(testCase.User, resp.User) {
			t.Errorf("%d: expected user %#v, got %#v", i, testCase.User, resp.User)
		}

		if testCase.Ok != ok {
			t.Errorf("%d: expected auth %v, got %v", i, testCase.Ok, ok)
		}
		switch {
		case err == nil && testCase.Err:
			t.Errorf("%d: unexpected nil error", i)
		case err != nil && !testCase.Err:
			t.Errorf("%d: unexpected error: %v", i, err)
		}
	}
}

func TestBadTokenFileMissingRequiredTenant(t *testing.T) {
	_, err := newWithContents(t, `
token1,user1,uid1
token2,user2,,
`)
	if err == nil {
		t.Fatalf("unexpected non error")
	}
}

func TestEmptyUidTokenFile(t *testing.T) {
	_, err := newWithContents(t, `
token1,user1,uid1
token2,user2,
`)
	if err == nil {
		t.Fatalf("unexpected non error")
	}
}

func TestBadTokenFile(t *testing.T) {
	_, err := newWithContents(t, `
token1,user1,uid1
token2,user2,uid2
token3,user3
token4
`)
	if err == nil {
		t.Fatalf("unexpected non error")
	}
}

func TestInsufficientColumnsTokenFile(t *testing.T) {
	_, err := newWithContents(t, "token4\n")
	if err == nil {
		t.Fatalf("unexpected non error")
	}
}

func TestEmptyTokenTokenFile(t *testing.T) {
	auth, err := newWithContents(t, ",user5,uid5\n")
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(auth.tokens) != 0 {
		t.Fatalf("empty token should not be recorded")
	}
}

func newWithContents(t *testing.T, contents string) (auth *TokenAuthenticator, err error) {
	f, err := ioutil.TempFile("", "tokenfile_test")
	if err != nil {
		t.Fatalf("unexpected error creating tokenfile: %v", err)
	}
	f.Close()
	defer os.Remove(f.Name())

	if err := ioutil.WriteFile(f.Name(), []byte(contents), 0700); err != nil {
		t.Fatalf("unexpected error writing tokenfile: %v", err)
	}

	return NewCSV(f.Name())
}
