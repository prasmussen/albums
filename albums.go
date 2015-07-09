package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	Version = "1.0.0"
)

var (
	UserAgent = fmt.Sprintf("albums/%s ( https://github.com/prasmussen/albums )", Version)
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "No artist provided")
		os.Exit(1)
	}

	query := strings.Join(os.Args[1:], " ")
	artist, err := findArtist(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}

	albums, err := findAlbums(artist.Id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}

	if len(albums) == 0 {
		fmt.Printf("%s has no albums yet\n", artist.Name)
		return
	}

	fmt.Printf("Albums by %s\n", artist.Name)
	for _, album := range albums {
		fmt.Printf("%04d %s\n", album.Year, album.Title)
	}
}

type ArtistResult struct {
	Artists []*Artist `json:"artists"`
}

type Artist struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type ReleaseGroupResult struct {
	ReleaseGroups []*ReleaseGroup `json:"release-groups"`
}

type ReleaseGroup struct {
	Id               string   `json:"id"`
	Title            string   `json:"title"`
	PrimaryType      string   `json:"primary-type"`
	SecondaryTypes   []string `json:"secondary-types"`
	FirstReleaseDate string   `json:"first-release-date"`
}

type Album struct {
	Title string
	Year  int
}

type AlbumByYear []*Album

func (a AlbumByYear) Len() int           { return len(a) }
func (a AlbumByYear) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a AlbumByYear) Less(i, j int) bool { return a[i].Year < a[j].Year }

func findArtist(name string) (*Artist, error) {
	query := url.Values{}
	query.Add("query", fmt.Sprintf("artist:%s", name))
	query.Add("limit", "1")
	query.Add("fmt", "json")

	req := &http.Request{
		Method: "GET",
		Host:   "musicbrainz.org",
		URL: &url.URL{
			Host:     "musicbrainz.org",
			Scheme:   "http",
			Path:     "/ws/2/artist/",
			RawQuery: query.Encode(),
		},
		Header: http.Header{
			"User-Agent": {UserAgent},
		},
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	artistResult := &ArtistResult{}
	err = json.NewDecoder(res.Body).Decode(&artistResult)
	if err != nil {
		return nil, err
	}

	if len(artistResult.Artists) == 0 {
		return nil, fmt.Errorf("No artists found")
	}

	return artistResult.Artists[0], nil
}

func findAlbums(artistId string) ([]*Album, error) {
	releaseGroups, err := findReleaseGroups(artistId)
	if err != nil {
		return nil, err
	}

	albums := make([]*Album, 0, 0)

	for _, rg := range releaseGroups {
		// Skip non-pure albums
		if len(rg.SecondaryTypes) > 0 {
			continue
		}

		albums = append(albums, &Album{
			Title: rg.Title,
			Year:  formatYear(rg.FirstReleaseDate),
		})
	}

	// Sort albums by year
	sort.Sort(AlbumByYear(albums))

	return albums, nil
}

func findReleaseGroups(artistId string) ([]*ReleaseGroup, error) {
	query := url.Values{}
	query.Add("artist", artistId)
	query.Add("type", "album")
	query.Add("limit", "100")
	query.Add("fmt", "json")

	req := &http.Request{
		Method: "GET",
		Host:   "musicbrainz.org",
		URL: &url.URL{
			Host:     "musicbrainz.org",
			Scheme:   "http",
			Path:     "/ws/2/release-group/",
			RawQuery: query.Encode(),
		},
		Header: http.Header{
			"User-Agent": {UserAgent},
		},
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	releaseGroupResult := &ReleaseGroupResult{}
	err = json.NewDecoder(res.Body).Decode(&releaseGroupResult)
	if err != nil {
		return nil, err
	}

	return releaseGroupResult.ReleaseGroups, nil
}

func formatYear(date string) int {
	re := regexp.MustCompile("^([0-9]{4})")
	matches := re.FindStringSubmatch(date)
	if len(matches) != 2 {
		return 0
	}

	year, _ := strconv.Atoi(matches[1])
	return year
}
