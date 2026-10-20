package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

type profileLoader func(context.Context) (profile, error)
type profile struct {
	Login, Name, Bio, Company, Location, Email, WebsiteURL, URL string
	Followers, Following                                        int
	Socials                                                     []socialLink
	Pins                                                        []pinnedRepository
	Calendar                                                    contributionCalendar
}
type socialLink struct{ DisplayName, Provider, URL string }
type pinnedRepository struct {
	NameWithOwner, Description, URL, Language string
	Private                                   bool
	Stars, Forks                              int
}
type contributionCalendar struct {
	StartedAt, EndedAt time.Time
	Total              int
	Weeks              []contributionWeek
}
type contributionWeek struct {
	FirstDay string
	Days     []contributionDay
}
type contributionDay struct {
	Date           string
	Weekday, Count int
	Level          string
}

const socialSelection = `socialAccounts(first:100%s) { nodes { displayName provider url } pageInfo { hasNextPage endCursor } }`
const pinSelection = `pinnedItems(first:100, types:[REPOSITORY]%s) { nodes { __typename ... on Repository { nameWithOwner description url isPrivate stargazerCount forkCount primaryLanguage { name } } } pageInfo { hasNextPage endCursor } }`

var profileQuery = `query Profile { viewer {
 login name bio company location websiteUrl url
 followers(first:1) { totalCount } following(first:1) { totalCount }
 ` + fmt.Sprintf(socialSelection, "") + `
 ` + fmt.Sprintf(pinSelection, "") + `
 contributionsCollection { startedAt endedAt contributionCalendar { totalContributions weeks { firstDay contributionDays { date weekday contributionCount contributionLevel } } } }
} }`

type pageInfoDTO struct {
	HasNextPage bool
	EndCursor   *string
}
type socialDTO struct {
	DisplayName   *string
	Provider, URL string
}
type pinDTO struct {
	Type                      string `json:"__typename"`
	NameWithOwner, URL        string
	Description               *string
	IsPrivate                 bool
	StargazerCount, ForkCount int
	PrimaryLanguage           *struct{ Name string }
}
type connectionDTO[T any] struct {
	Nodes    []*T
	PageInfo pageInfoDTO
}
type calendarDTO struct {
	TotalContributions int
	Weeks              []struct {
		FirstDay         string
		ContributionDays []struct {
			Date                       string
			Weekday, ContributionCount int
			ContributionLevel          string
		}
	}
}
type viewerDTO struct {
	Login                                    string
	Name, Bio, Company, Location, WebsiteURL *string
	URL                                      string
	Followers, Following                     struct{ TotalCount int }
	SocialAccounts                           connectionDTO[socialDTO]
	PinnedItems                              connectionDTO[pinDTO]
	ContributionsCollection                  *struct {
		StartedAt, EndedAt   string
		ContributionCalendar *calendarDTO
	}
}
type profileResponse struct{ Viewer *viewerDTO }

func optional(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func loadProfile(ctx context.Context) (profile, error) {
	opts := api.ClientOptions{Timeout: 20 * time.Second, LogIgnoreEnv: true}
	graphql, err := api.NewGraphQLClient(opts)
	if err != nil {
		return profile{}, fmt.Errorf("create GraphQL client: %w", err)
	}
	rest, err := api.NewRESTClient(opts)
	if err != nil {
		return profile{}, fmt.Errorf("create REST client: %w", err)
	}
	return fetchProfile(ctx, graphql, rest)
}

func fetchProfile(ctx context.Context, graphql *api.GraphQLClient, rest *api.RESTClient) (profile, error) {
	var response profileResponse
	if err := graphql.DoWithContext(ctx, profileQuery, nil, &response); err != nil {
		return profile{}, fmt.Errorf("load profile: %w", err)
	}
	v := response.Viewer
	if v == nil || v.Login == "" {
		return profile{}, fmt.Errorf("load profile: missing viewer or login")
	}
	var publicUser struct {
		Login string
		Email *string
	}
	if err := rest.DoWithContext(ctx, http.MethodGet, "users/"+url.PathEscape(v.Login), nil, &publicUser); err != nil {
		return profile{}, fmt.Errorf("load public profile email: %w", err)
	}
	if !strings.EqualFold(publicUser.Login, v.Login) {
		return profile{}, fmt.Errorf("load public profile email: returned login does not match viewer")
	}
	socials, err := collectPages(ctx, graphql, v.SocialAccounts, socialSelection, func(v *viewerDTO) connectionDTO[socialDTO] { return v.SocialAccounts })
	if err != nil {
		return profile{}, fmt.Errorf("load social accounts: %w", err)
	}
	pins, err := collectPages(ctx, graphql, v.PinnedItems, pinSelection, func(v *viewerDTO) connectionDTO[pinDTO] { return v.PinnedItems })
	if err != nil {
		return profile{}, fmt.Errorf("load pinned repositories: %w", err)
	}
	c, err := normalizeCalendar(v)
	if err != nil {
		return profile{}, fmt.Errorf("load contribution calendar: %w", err)
	}
	p := profile{Login: v.Login, Name: optional(v.Name), Bio: optional(v.Bio), Company: optional(v.Company), Location: optional(v.Location), Email: optional(publicUser.Email), WebsiteURL: optional(v.WebsiteURL), URL: v.URL, Followers: v.Followers.TotalCount, Following: v.Following.TotalCount, Calendar: c}
	for _, s := range socials {
		p.Socials = append(p.Socials, socialLink{optional(s.DisplayName), s.Provider, s.URL})
	}
	for _, r := range pins {
		if r.Type != "Repository" {
			continue
		}
		language := ""
		if r.PrimaryLanguage != nil {
			language = r.PrimaryLanguage.Name
		}
		p.Pins = append(p.Pins, pinnedRepository{r.NameWithOwner, optional(r.Description), r.URL, language, r.IsPrivate, r.StargazerCount, r.ForkCount})
	}
	return p, nil
}

func collectPages[T any](ctx context.Context, client *api.GraphQLClient, page connectionDTO[T], selection string, connection func(*viewerDTO) connectionDTO[T]) ([]*T, error) {
	var nodes []*T
	seen := make(map[string]bool)
	for {
		for _, n := range page.Nodes {
			if n != nil {
				nodes = append(nodes, n)
			}
		}
		if !page.PageInfo.HasNextPage {
			return nodes, nil
		}
		cursor := optional(page.PageInfo.EndCursor)
		if cursor == "" || seen[cursor] {
			return nil, fmt.Errorf("missing or repeated pagination cursor")
		}
		seen[cursor] = true
		var response profileResponse
		query := `query ProfilePage($after:String) { viewer { ` + fmt.Sprintf(selection, ", after:$after") + ` } }`
		if err := client.DoWithContext(ctx, query, map[string]interface{}{"after": cursor}, &response); err != nil {
			return nil, err
		}
		if response.Viewer == nil {
			return nil, fmt.Errorf("missing viewer on pagination page")
		}
		page = connection(response.Viewer)
	}
}

func normalizeCalendar(v *viewerDTO) (contributionCalendar, error) {
	var result contributionCalendar
	collection := v.ContributionsCollection
	if collection == nil || collection.ContributionCalendar == nil {
		return result, fmt.Errorf("missing required calendar")
	}
	var err error
	result.StartedAt, err = time.Parse(time.RFC3339, collection.StartedAt)
	if err != nil {
		return result, fmt.Errorf("invalid start: %w", err)
	}
	result.EndedAt, err = time.Parse(time.RFC3339, collection.EndedAt)
	if err != nil {
		return result, fmt.Errorf("invalid end: %w", err)
	}
	result.Total = collection.ContributionCalendar.TotalContributions
	seen := make(map[string]bool)
	for _, w := range collection.ContributionCalendar.Weeks {
		first, err := time.Parse(time.DateOnly, w.FirstDay)
		if err != nil {
			return result, fmt.Errorf("invalid week date: %w", err)
		}
		if first.Weekday() != time.Sunday {
			return result, fmt.Errorf("week does not start on Sunday: %s", w.FirstDay)
		}
		week := contributionWeek{FirstDay: w.FirstDay}
		for _, d := range w.ContributionDays {
			date, err := time.Parse(time.DateOnly, d.Date)
			if err != nil {
				return result, fmt.Errorf("invalid contribution date: %w", err)
			}
			if int(date.Weekday()) != d.Weekday || date.Before(first) || !date.Before(first.AddDate(0, 0, 7)) {
				return result, fmt.Errorf("date/weekday/week mismatch: %s", d.Date)
			}
			if seen[d.Date] {
				return result, fmt.Errorf("duplicate contribution date: %s", d.Date)
			}
			seen[d.Date] = true
			week.Days = append(week.Days, contributionDay{d.Date, d.Weekday, d.ContributionCount, d.ContributionLevel})
		}
		result.Weeks = append(result.Weeks, week)
	}
	return result, nil
}
