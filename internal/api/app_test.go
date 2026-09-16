package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const publicGroup = `{"id":12,"appID":2555,"name":"Families","description":"News","anyoneCanJoin":1,"invitationOnly":0,"askToJoin":0,"loginRequired":0,"hidden":0,"pwd":"","memberCnt":9}`

func appServer(t *testing.T, handler http.HandlerFunc) *AppClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := NewApp()
	c.client.baseURL = server.URL
	return c
}

func TestAppJSONResources(t *testing.T) {
	c := appServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Error("missing JSON Accept")
		}
		if r.Header.Get("Cookie") != "" || r.Header.Get("X-CSRF-Token") != "" || r.URL.Query().Has("account_id") {
			t.Error("portal authentication crossed services")
		}
		switch r.URL.Path {
		case "/getAppInfo.php":
			if r.URL.Query().Get("appid") != "2555" {
				t.Error("wrong app scope")
			}
			w.Write([]byte(`{"appinfo":{"appName":"Dance","iosVersion":"1.2","Android_Link":"https://example.test/app"},"locations":[{"id":"1","name":"Studio","cmsID":"30834","cmsType":"dancestudiopro"}],"secret":"do-not-print"}`))
		case "/groupsUtil.php":
			if r.URL.Query().Get("req") != "getgroups" || r.URL.Query().Get("appID") != "2555" {
				t.Error("wrong group query")
			}
			w.Write([]byte("[" + publicGroup + "]"))
		case "/notificationsUtil.php":
			if r.URL.Query().Get("req") != "getMsgs" || r.URL.Query().Get("appID") != "2555" || r.URL.Query().Get("groupID") != "12" {
				t.Error("wrong notification query")
			}
			w.Write([]byte(`{"messages":[{"PushMsgId":3,"sentDt":123,"title":"News","Msg":"Hello","detail":"PHA+SGk8L3A+","PushPage":"aHR0cHM6Ly9leGFtcGxlLnRlc3Q=","mediaSrc":"","chatToken":"do-not-print"}]}`))
		case "/AppProfileManager.php":
			r.ParseForm()
			if r.Method != "POST" || r.PostForm.Get("req") != "getacctinfo" || r.PostForm.Get("aid") != "2555" || r.PostForm.Get("email") != "parent@example.test" || len(r.PostForm) != 3 {
				t.Error("wrong profile request")
			}
			w.Write([]byte(`{"success":true,"data":{"id":"7","firstName":"Parent","email":"parent@example.test","chatToken":"do-not-print","password":"do-not-print"}}`))
		default:
			t.Fatalf("unexpected endpoint %s", r.URL.Path)
		}
	})
	ctx := context.Background()
	info, err := c.Info(ctx)
	if err != nil || info.Name != "Dance" || info.Locations[0].AccountID != "30834" {
		t.Fatalf("info: %v %v", info, err)
	}
	groups, err := c.Groups(ctx)
	if err != nil || len(groups) != 1 || !groups[0].Public {
		t.Fatalf("groups: %v %v", groups, err)
	}
	msgs, err := c.Notifications(ctx, 12)
	if err != nil || len(msgs) != 1 || msgs[0].DetailHTML != "<p>Hi</p>" || msgs[0].Link != "https://example.test" {
		t.Fatalf("notifications: %v %v", msgs, err)
	}
	profile, err := c.Profile(ctx, "parent@example.test")
	if err != nil || profile.FirstName != "Parent" {
		t.Fatalf("profile: %v %v", profile, err)
	}
	for _, v := range []any{info, groups, msgs, profile} {
		b, _ := json.Marshal(v)
		if strings.Contains(string(b), "do-not-print") {
			t.Fatal("unapproved field leaked")
		}
	}
}

func TestAppRejectsRestrictedGroups(t *testing.T) {
	for _, replacement := range []string{`"pwd":"secret"`, `"invitationOnly":1`, `"askToJoin":1`, `"loginRequired":1`, `"anyoneCanJoin":0`, `"hidden":1`} {
		t.Run(replacement, func(t *testing.T) {
			field := strings.Split(replacement, ":")[0]
			original := field + ":0"
			if field == `"pwd"` {
				original = field + `:""`
			}
			if field == `"anyoneCanJoin"` {
				original = field + ":1"
			}
			c := appServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/groupsUtil.php" {
					t.Fatal("attempted private message access")
				}
				w.Write([]byte("[" + strings.Replace(publicGroup, original, replacement, 1) + "]"))
			})
			if _, err := c.Notifications(context.Background(), 12); err == nil {
				t.Fatal("accepted restricted group")
			}
		})
	}
}

func TestAppRejectsInvalidResponses(t *testing.T) {
	for _, body := range []string{`<html>secret</html>`, `null`, `{}`, `{"error":"secret"}`, `[null]`, `[{"id":12,"name":"Missing access rules"}]`} {
		c := appServer(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) })
		if _, err := c.Info(context.Background()); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("invalid app info accepted or leaked")
		}
		_, err := c.Groups(context.Background())
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("invalid group response accepted or leaked: %v", err)
		}
	}
	for _, body := range []string{`null`, `{}`, `{"success":false,"message":"secret"}`, `{"success":true,"data":{"id":"7","email":"other@example.test"}}`} {
		c := appServer(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) })
		if _, err := c.Profile(context.Background(), "parent@example.test"); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("invalid profile accepted or leaked")
		}
	}
}

func TestAppEmptyAndFailedMessages(t *testing.T) {
	for _, body := range []string{`{"messages":[]}`, `{"messages":null}`, `{}`, `{"messages":[{"PushMsgId":1,"sentDt":123,"detail":"not base64"}]}`, `{"messages":[{}]}`, `<html>secret</html>`} {
		c := appServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/groupsUtil.php" {
				w.Write([]byte("[" + publicGroup + "]"))
				return
			}
			w.Write([]byte(body))
		})
		msgs, err := c.Notifications(context.Background(), 12)
		if body == `{"messages":[]}` {
			if err != nil || msgs == nil || len(msgs) != 0 {
				t.Fatal("empty response failed")
			}
		} else if err == nil {
			t.Fatal("accepted invalid messages")
		}
	}
}

func TestAppEmptyGroups(t *testing.T) {
	c := appServer(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[]`)) })
	groups, err := c.Groups(context.Background())
	if err != nil || groups == nil || len(groups) != 0 {
		t.Fatal("empty groups must return an empty array")
	}
}

func TestAppRejectsUnknownGroupAndDryRun(t *testing.T) {
	calls := 0
	c := appServer(t, func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte("[" + publicGroup + "]")) })
	if _, err := c.Notifications(context.Background(), 0); err == nil || calls != 0 {
		t.Fatal("invalid ID sent")
	}
	if _, err := c.Notifications(context.Background(), 99); err == nil || calls != 1 {
		t.Fatal("unknown group accepted")
	}
	c.client.dryRun = true
	if _, err := c.Profile(context.Background(), "parent@example.test"); err == nil || calls != 1 {
		t.Fatal("dry-run sent a POST")
	}
}
