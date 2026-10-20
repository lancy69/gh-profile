package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func jsonViewer(v map[string]any) (*viewerDTO, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var result viewerDTO
	err = json.Unmarshal(b, &result)
	return &result, err
}
func fixtureViewer() map[string]any {
	return map[string]any{
		"login": "tester", "name": "Test User", "bio": "Biography", "company": "Company", "location": "Earth", "websiteUrl": "https://example.com", "url": "https://github.com/tester",
		"followers": map[string]any{"totalCount": 4}, "following": map[string]any{"totalCount": 2},
		"socialAccounts": map[string]any{"nodes": []any{nil, map[string]any{"displayName": "Social", "provider": "MASTODON", "url": "https://social.example/@test"}}},
		"pinnedItems":    map[string]any{"nodes": []any{nil, map[string]any{"__typename": "Issue"}, map[string]any{"__typename": "Repository", "nameWithOwner": "owner/project", "description": "Description", "url": "https://github.com/owner/project", "isPrivate": true, "stargazerCount": 1234, "forkCount": 5, "primaryLanguage": map[string]any{"name": "Go"}}}},
		"contributionsCollection": map[string]any{"startedAt": "2024-02-28T00:00:00Z", "endedAt": "2024-03-05T23:59:59Z", "contributionCalendar": map[string]any{"totalContributions": 5, "weeks": []any{
			map[string]any{"firstDay": "2024-02-25", "contributionDays": []any{map[string]any{"date": "2024-02-28", "weekday": 3, "contributionCount": 0, "contributionLevel": "NONE"}, map[string]any{"date": "2024-02-29", "weekday": 4, "contributionCount": 5, "contributionLevel": "FOURTH_QUARTILE"}}},
			map[string]any{"firstDay": "2024-03-03", "contributionDays": []any{map[string]any{"date": "2024-03-05", "weekday": 2, "contributionCount": 0, "contributionLevel": "NONE"}}},
		}}},
	}
}
func testClients(t *testing.T, f roundTripFunc) (*api.GraphQLClient, *api.RESTClient) {
	t.Helper()
	options := api.ClientOptions{Host: "github.com", AuthToken: "test-token", Transport: f, LogIgnoreEnv: true}
	g, e := api.NewGraphQLClient(options)
	if e != nil {
		t.Fatal(e)
	}
	r, e := api.NewRESTClient(options)
	if e != nil {
		t.Fatal(e)
	}
	return g, r
}
func jsonResponse(body any, status int) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(b)))}, nil
}
func TestFetchProfile(t *testing.T) {
	v := fixtureViewer()
	g, r := testClients(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			if req.URL.Path != "/users/tester" {
				t.Fatalf("unexpected public endpoint: %s", req.URL.Path)
			}
			return jsonResponse(map[string]any{"login": "TESTER", "email": "public@example.com"}, 200)
		}
		var body struct{ Query string }
		json.NewDecoder(req.Body).Decode(&body)
		if strings.Contains(body.Query, "email") {
			t.Fatal("email must not require GraphQL private-email scope")
		}
		return jsonResponse(map[string]any{"data": map[string]any{"viewer": v}}, 200)
	})
	p, err := fetchProfile(context.Background(), g, r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Test User" || p.Email != "public@example.com" || p.Followers != 4 || p.Following != 2 || len(p.Socials) != 1 || len(p.Pins) != 1 || p.Pins[0].Language != "Go" || !p.Pins[0].Private || p.Pins[0].Stars != 1234 || p.Calendar.Total != 5 || p.Calendar.Weeks[0].Days[1].Date != "2024-02-29" {
		t.Fatalf("incorrect normalized profile: %+v", p)
	}
}
func TestFetchEmptyOptionalFields(t *testing.T) {
	v := fixtureViewer()
	v["name"] = nil
	v["bio"] = nil
	v["socialAccounts"] = map[string]any{}
	v["pinnedItems"] = map[string]any{"nodes": []any{map[string]any{"__typename": "Repository", "nameWithOwner": "a/b", "primaryLanguage": nil}}}
	c := v["contributionsCollection"].(map[string]any)["contributionCalendar"].(map[string]any)
	c["totalContributions"] = 0
	c["weeks"] = []any{}
	g, r := testClients(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == "GET" {
			return jsonResponse(map[string]any{"login": "tester", "email": nil, "notification_email": "private@example.com"}, 200)
		}
		return jsonResponse(map[string]any{"data": map[string]any{"viewer": v}}, 200)
	})
	p, err := fetchProfile(context.Background(), g, r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "" || p.Bio != "" || p.Email != "" || p.Pins[0].Language != "" || len(p.Socials) != 0 || p.Calendar.Total != 0 {
		t.Fatalf("optional fields: %+v", p)
	}
}
func TestPagination(t *testing.T) {
	for _, bad := range []string{"", "missing", "repeated"} {
		t.Run("cursor_"+bad, func(t *testing.T) {
			v := fixtureViewer()
			for _, key := range []string{"socialAccounts", "pinnedItems"} {
				v[key].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": true, "endCursor": "one"}
			}
			if bad == "missing" {
				v["socialAccounts"].(map[string]any)["pageInfo"] = map[string]any{"hasNextPage": true}
			}
			g, r := testClients(t, func(req *http.Request) (*http.Response, error) {
				if req.Method == "GET" {
					return jsonResponse(map[string]any{"login": "tester"}, 200)
				}
				var b struct {
					Query     string
					Variables map[string]any
				}
				json.NewDecoder(req.Body).Decode(&b)
				viewer := v
				if strings.Contains(b.Query, "ProfilePage") {
					if strings.Contains(b.Query, "contributionsCollection") {
						t.Fatal("refetched calendar")
					}
					if b.Variables["after"] != "one" {
						t.Fatal("lost cursor")
					}
					key := "pinnedItems"
					node := map[string]any{"__typename": "Repository", "nameWithOwner": "second/pin"}
					if strings.Contains(b.Query, "socialAccounts") {
						key = "socialAccounts"
						node = map[string]any{"displayName": "Second", "url": "https://second.example"}
					}
					viewer = map[string]any{key: map[string]any{"nodes": []any{nil, node}, "pageInfo": map[string]any{"hasNextPage": bad == "repeated", "endCursor": "one"}}}
				}
				return jsonResponse(map[string]any{"data": map[string]any{"viewer": viewer}}, 200)
			})
			p, err := fetchProfile(context.Background(), g, r)
			if bad != "" {
				if err == nil {
					t.Fatal("accepted invalid cursor")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Socials) != 2 || p.Socials[1].DisplayName != "Second" || len(p.Pins) != 2 || p.Pins[1].NameWithOwner != "second/pin" {
				t.Fatalf("lost pagination order: %+v", p)
			}
		})
	}
}
func TestLoadFailures(t *testing.T) {
	for _, kind := range []string{"partial", "viewer", "login", "calendar", "rest", "mismatch", "date", "weekday", "duplicate", "period"} {
		t.Run(kind, func(t *testing.T) {
			v := fixtureViewer()
			response := map[string]any{"data": map[string]any{"viewer": v}}
			switch kind {
			case "partial":
				response["errors"] = []any{map[string]any{"message": "denied"}}
			case "viewer":
				response["data"] = map[string]any{"viewer": nil}
			case "login":
				v["login"] = ""
			case "calendar":
				v["contributionsCollection"] = nil
			case "period":
				v["contributionsCollection"].(map[string]any)["startedAt"] = "invalid"
			case "date", "weekday", "duplicate":
				weeks := v["contributionsCollection"].(map[string]any)["contributionCalendar"].(map[string]any)["weeks"].([]any)
				w := weeks[0].(map[string]any)
				days := w["contributionDays"].([]any)
				day := days[0].(map[string]any)
				if kind == "date" {
					day["date"] = "2024-02-30"
				}
				if kind == "weekday" {
					day["weekday"] = 2
				}
				if kind == "duplicate" {
					w["contributionDays"] = append(days, day)
				}
			}
			g, r := testClients(t, func(req *http.Request) (*http.Response, error) {
				if req.Method == "GET" {
					if kind == "rest" {
						return nil, fmt.Errorf("transport unavailable")
					}
					login := "tester"
					if kind == "mismatch" {
						login = "wrong"
					}
					return jsonResponse(map[string]any{"login": login}, 200)
				}
				return jsonResponse(response, 200)
			})
			if _, err := fetchProfile(context.Background(), g, r); err == nil {
				t.Fatal("accepted incomplete or invalid profile")
			}
		})
	}
}
func TestLoadCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g, r := testClients(t, func(req *http.Request) (*http.Response, error) { return nil, req.Context().Err() })
	_, err := fetchProfile(ctx, g, r)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
}
