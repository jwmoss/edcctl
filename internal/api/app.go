package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const EDCAppID = 2555

// AppClient has its own transport and cookie jar. Portal credentials never cross hosts.
type AppClient struct{ client *Client }

func NewApp(opts ...Option) *AppClient {
	return &AppClient{client: New("https://mobileinventor.com/applib", "", opts...)}
}

func (c *AppClient) call(ctx context.Context, method, path string, form url.Values, out any) error {
	data, err := c.client.do(ctx, method, path, form, "application/json")
	if err != nil {
		return err
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) || json.Unmarshal(data, out) != nil {
		return fmt.Errorf("app endpoint did not return the expected JSON response")
	}
	return nil
}

type AppLocation struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AccountID string `json:"cmsID"`
	Backend   string `json:"cmsType"`
}

type AppInfo struct {
	AppID          int           `json:"app_id"`
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	IOSVersion     string        `json:"ios_version"`
	AndroidVersion string        `json:"android_version"`
	IOSURL         string        `json:"ios_url"`
	AndroidURL     string        `json:"android_url"`
	Locations      []AppLocation `json:"locations"`
}

func (c *AppClient) Info(ctx context.Context) (*AppInfo, error) {
	var wire struct {
		AppInfo struct {
			Name           string `json:"appName"`
			Description    string `json:"appDesc"`
			IOSVersion     string `json:"iosVersion"`
			AndroidVersion string `json:"androidVersion"`
			IOSURL         string `json:"Itunes_Link"`
			AndroidURL     string `json:"Android_Link"`
		} `json:"appinfo"`
		Locations []AppLocation `json:"locations"`
	}
	if err := c.call(ctx, http.MethodGet, "/getAppInfo.php?appid="+strconv.Itoa(EDCAppID), nil, &wire); err != nil {
		return nil, err
	}
	if wire.AppInfo.Name == "" || wire.Locations == nil {
		return nil, fmt.Errorf("app info response is incomplete")
	}
	return &AppInfo{AppID: EDCAppID, Name: wire.AppInfo.Name, Description: wire.AppInfo.Description, IOSVersion: wire.AppInfo.IOSVersion, AndroidVersion: wire.AppInfo.AndroidVersion, IOSURL: wire.AppInfo.IOSURL, AndroidURL: wire.AppInfo.AndroidURL, Locations: wire.Locations}, nil
}

type AppGroup struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
	MemberCount int    `json:"member_count"`
}

func (c *AppClient) Groups(ctx context.Context) ([]AppGroup, error) {
	var wire []struct {
		ID             int     `json:"id"`
		AppID          int     `json:"appID"`
		Name           string  `json:"name"`
		Description    string  `json:"description"`
		AnyoneCanJoin  *int    `json:"anyoneCanJoin"`
		InvitationOnly *int    `json:"invitationOnly"`
		AskToJoin      *int    `json:"askToJoin"`
		LoginRequired  *int    `json:"loginRequired"`
		Hidden         *int    `json:"hidden"`
		Password       *string `json:"pwd"`
		MemberCount    int     `json:"memberCnt"`
	}
	if err := c.call(ctx, http.MethodGet, "/groupsUtil.php?req=getgroups&appID="+strconv.Itoa(EDCAppID), nil, &wire); err != nil {
		return nil, err
	}
	if wire == nil {
		return nil, fmt.Errorf("app groups response is not an array")
	}
	groups := make([]AppGroup, 0, len(wire))
	for _, g := range wire {
		// Fail closed if the server omits access rules. Never expose group passwords or member lists.
		if g.ID <= 0 || g.AppID != EDCAppID || g.Name == "" || g.AnyoneCanJoin == nil || g.InvitationOnly == nil || g.AskToJoin == nil || g.LoginRequired == nil || g.Hidden == nil || g.Password == nil {
			return nil, fmt.Errorf("app group response lacks identity or access rules")
		}
		if *g.Hidden != 0 {
			continue
		}
		public := *g.AnyoneCanJoin == 1 && *g.InvitationOnly == 0 && *g.AskToJoin == 0 && *g.LoginRequired == 0 && *g.Password == ""
		groups = append(groups, AppGroup{ID: g.ID, Name: g.Name, Description: g.Description, Public: public, MemberCount: g.MemberCount})
	}
	return groups, nil
}

type AppNotification struct {
	ID         int    `json:"id"`
	SentAt     int64  `json:"sent_at"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	DetailHTML string `json:"detail_html"`
	ButtonText string `json:"button_text"`
	LinkType   string `json:"link_type"`
	Link       string `json:"link"`
	MediaType  string `json:"media_type"`
	MediaURL   string `json:"media_url"`
}

func (c *AppClient) Notifications(ctx context.Context, groupID int) ([]AppNotification, error) {
	if groupID <= 0 {
		return nil, fmt.Errorf("group ID must be positive")
	}
	groups, err := c.Groups(ctx)
	if err != nil {
		return nil, err
	}
	public := false
	for _, g := range groups {
		if g.ID == groupID {
			public = g.Public
			break
		}
	}
	if !public {
		return nil, fmt.Errorf("group is unknown or restricted; private-group membership is not verified")
	}
	var wire struct {
		Messages []struct {
			ID         int    `json:"PushMsgId"`
			SentAt     int64  `json:"sentDt"`
			Title      string `json:"title"`
			Message    string `json:"Msg"`
			Detail     string `json:"detail"`
			ButtonText string `json:"buttonText"`
			LinkType   string `json:"pushType"`
			Link       string `json:"PushPage"`
			MediaType  string `json:"mediaType"`
			MediaURL   string `json:"mediaSrc"`
		} `json:"messages"`
	}
	path := "/notificationsUtil.php?req=getMsgs&appID=" + strconv.Itoa(EDCAppID) + "&groupID=" + strconv.Itoa(groupID)
	if err := c.call(ctx, http.MethodGet, path, nil, &wire); err != nil {
		return nil, err
	}
	if wire.Messages == nil {
		return nil, fmt.Errorf("app notifications response lacks a message array")
	}
	messages := make([]AppNotification, 0, len(wire.Messages))
	for _, m := range wire.Messages {
		detail, e1 := base64.StdEncoding.DecodeString(m.Detail)
		link, e2 := base64.StdEncoding.DecodeString(m.Link)
		media, e3 := base64.StdEncoding.DecodeString(m.MediaURL)
		if m.ID <= 0 || m.SentAt <= 0 || e1 != nil || e2 != nil || e3 != nil {
			return nil, fmt.Errorf("app notification contains invalid fields")
		}
		// The JSON API encodes rich text as base64. Preserve it without parsing or rendering HTML.
		messages = append(messages, AppNotification{ID: m.ID, SentAt: m.SentAt, Title: m.Title, Message: m.Message, DetailHTML: string(detail), ButtonText: m.ButtonText, LinkType: m.LinkType, Link: string(link), MediaType: m.MediaType, MediaURL: string(media)})
	}
	return messages, nil
}

type AppProfile struct {
	ID          string `json:"id"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Email       string `json:"email"`
	DateOfBirth string `json:"dob,omitempty"`
	SpotTVEmail string `json:"spotTvEmail,omitempty"`
}

// Profile reads only the caller's email, after the CLI verifies the Studio Pro login.
// It deliberately omits chattoken=1: that option requests a credential, not profile data.
func (c *AppClient) Profile(ctx context.Context, email string) (*AppProfile, error) {
	if strings.TrimSpace(email) == "" {
		return nil, fmt.Errorf("profile email is required")
	}
	var wire struct {
		Success bool        `json:"success"`
		Data    *AppProfile `json:"data"`
	}
	if err := c.call(ctx, http.MethodPost, "/AppProfileManager.php", url.Values{"req": {"getacctinfo"}, "aid": {strconv.Itoa(EDCAppID)}, "email": {email}}, &wire); err != nil {
		return nil, err
	}
	if !wire.Success || wire.Data == nil || wire.Data.ID == "" || !strings.EqualFold(wire.Data.Email, email) {
		return nil, fmt.Errorf("app profile is unavailable for the authenticated email")
	}
	return wire.Data, nil
}
