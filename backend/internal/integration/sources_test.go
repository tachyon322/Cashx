package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type sourceItem struct {
	ID        string  `json:"id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	GroupID   *string `json:"group_id"`
	GroupName *string `json:"group_name"`
	IsDefault bool    `json:"is_default"`
	IsActive  bool    `json:"is_active"`
	URL       string  `json:"url"`
}

func TestSourcesCRUD(t *testing.T) {
	pool := setup(t)
	data, apiSrv, redirSrv := fullSetup(t, pool)
	base := apiSrv.URL + "/api/v1/cabinet"

	// Initially: one default source created by Join.
	resp, body := doJSON(t, data.Partner, "GET", base+"/offers/"+data.Offer+"/sources", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list sources: %d %s", resp.StatusCode, body)
	}
	var list struct {
		Items []sourceItem `json:"items"`
	}
	_ = json.Unmarshal(body, &list)
	if len(list.Items) != 1 || !list.Items[0].IsDefault {
		t.Fatalf("expected 1 default source, got %+v", list.Items)
	}
	defID, defName, defCode := list.Items[0].ID, list.Items[0].Name, list.Items[0].Code

	// Create a group.
	resp, body = doJSON(t, data.Partner, "POST", base+"/source-groups", map[string]any{
		"name": "Telegram", "comment": "основной канал",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create group: %d %s", resp.StatusCode, body)
	}
	var group struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &group)

	// Create a source with a custom code, bound to the group.
	resp, body = doJSON(t, data.Partner, "POST", base+"/offers/"+data.Offer+"/sources", map[string]any{
		"name": "Telegram #1", "code": "tg1", "group_id": group.ID,
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create source: %d %s", resp.StatusCode, body)
	}
	var tg1 sourceItem
	_ = json.Unmarshal(body, &tg1)
	if tg1.Code != "TG1" {
		t.Fatalf("code should be normalized to TG1, got %q", tg1.Code)
	}
	if tg1.GroupID == nil || *tg1.GroupID != group.ID {
		t.Fatalf("group_id not set: %+v", tg1)
	}
	if !strings.HasSuffix(tg1.URL, "/c/TG1") {
		t.Fatalf("unexpected url: %s", tg1.URL)
	}

	// Duplicate code -> 409.
	resp, body = doJSON(t, data.Partner, "POST", base+"/offers/"+data.Offer+"/sources", map[string]any{
		"name": "Telegram #2", "code": "TG1",
	}, nil)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(body), "code_taken") {
		t.Fatalf("duplicate code: want 409 code_taken, got %d %s", resp.StatusCode, body)
	}

	// Custom code resolves through the redirect and records a click.
	redir := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := redir.Get(redirSrv.URL + "/c/TG1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("custom code redirect: want 302, got %d", resp.StatusCode)
	}

	// Rename works.
	resp, body = doJSON(t, data.Partner, "PATCH", base+"/offers/"+data.Offer+"/sources/"+tg1.ID, map[string]any{
		"name": "Telegram renamed", "code": "tg1", "group_id": group.ID, "is_active": true, "is_default": false,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update source: %d %s", resp.StatusCode, body)
	}
	var updated sourceItem
	_ = json.Unmarshal(body, &updated)
	if updated.Name != "Telegram renamed" {
		t.Fatalf("unexpected update result: %+v", updated)
	}

	// Deleting a source that already has a click -> 409 has_clicks.
	resp, body = doJSON(t, data.Partner, "DELETE", base+"/offers/"+data.Offer+"/sources/"+tg1.ID, nil, nil)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(body), "has_clicks") {
		t.Fatalf("delete clicked source: want 409 has_clicks, got %d %s", resp.StatusCode, body)
	}

	// A clickless source can be deleted.
	resp, body = doJSON(t, data.Partner, "POST", base+"/offers/"+data.Offer+"/sources", map[string]any{
		"name": "Temporary", "code": "TEMP",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create temp source: %d %s", resp.StatusCode, body)
	}
	var temp sourceItem
	_ = json.Unmarshal(body, &temp)
	resp, body = doJSON(t, data.Partner, "DELETE", base+"/offers/"+data.Offer+"/sources/"+temp.ID, nil, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete temp source: %d %s", resp.StatusCode, body)
	}

	// Group with sources cannot be deleted.
	resp, body = doJSON(t, data.Partner, "DELETE", base+"/source-groups/"+group.ID, nil, nil)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(body), "group_not_empty") {
		t.Fatalf("delete non-empty group: want 409 group_not_empty, got %d %s", resp.StatusCode, body)
	}

	// Unbind the source from the group, then the group deletes cleanly.
	resp, body = doJSON(t, data.Partner, "PATCH", base+"/offers/"+data.Offer+"/sources/"+tg1.ID, map[string]any{
		"name": "Telegram renamed", "code": "tg1", "group_id": nil, "is_active": true, "is_default": false,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unbind source: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, data.Partner, "DELETE", base+"/source-groups/"+group.ID, nil, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete group: %d %s", resp.StatusCode, body)
	}

	// The last active source cannot be deactivated.
	resp, body = doJSON(t, data.Partner, "PATCH", base+"/offers/"+data.Offer+"/sources/"+tg1.ID, map[string]any{
		"name": "Telegram renamed", "code": "tg1", "is_active": false, "is_default": false,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("deactivate non-last source: %d %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, data.Partner, "PATCH", base+"/offers/"+data.Offer+"/sources/"+defID, map[string]any{
		"name": defName, "code": defCode, "is_active": false, "is_default": true,
	}, nil)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(body), "last_source") {
		t.Fatalf("deactivate last source: want 409 last_source, got %d %s", resp.StatusCode, body)
	}
}
