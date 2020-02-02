package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var (
	envAPIKey     = requiredEnv("UNCANNIFIER_API_KEY")
	envMasterList = requiredEnv("UNCANNIFIER_MASTER_LIST")
	envSplit      = defaultEnvInt("UNCANNIFIER_SPLIT", 10)
)

func requiredEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		log.Fatalf("%q must be a defined environment variable", key)
	}
	return v
}

func defaultEnvInt(key string, val int) int {
	v, ok := os.LookupEnv(key)
	if ok {
		vv, err := strconv.Atoi(v)
		if err != nil {
			log.Fatal("expected number, got", v)
		}
		return vv
	}
	return val
}

func main() {
	if err := do(); err != nil {
		log.Fatal(err)
	}
	log.Println("completed successfully")
}

func do() error {
	ctx := context.Background()
	c, err := newClient(ctx, envAPIKey)
	if err != nil {
		return fmt.Errorf("Couldn't create client: %v", err)
	}
	l, err := c.list(ctx, envMasterList)
	if err != nil {
		return fmt.Errorf("Couldn't get playlist contents: %v", err)
	}
	fmt.Fprintln(os.Stderr, "total length:", len(l))
	ll, err := split(l, envSplit)
	if err != nil {
		return fmt.Errorf("Couldn't split playlist: %v", err)
	}
	for i, x := range ll {
		fmt.Fprintf(os.Stderr, "sublist %d length %d. Contents: %v\n", i, len(x), x)
	}
	for i, sub := range ll {
		if err := c.publish(ctx, playlistname(i), sub); err != nil {
			return err
		}
	}
	return nil
}

func playlistname(i int) string {
	x := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf("uncanny_%s_%d", x, i)
}

type client struct {
	y *youtube.Service
}

func newClient(ctx context.Context, key string) (*client, error) {
	service, err := youtube.NewService(ctx, option.WithAPIKey(key))
	if err != nil {
		return nil, err
	}
	return &client{service}, nil
}

func (c *client) list(ctx context.Context, id string) ([]string, error) {
	var vv []string
	next := ""
	for {
		// Retrieve next set of items in the playlist.
		ss, nextToken, err := c.snippet(id, next)
		if err != nil {
			return nil, err
		}

		vv = append(vv, ss...)
		fmt.Fprint(os.Stderr, ".")

		// Set the token to retrieve the next page of results
		// or exit the loop if all results have been retrieved.
		next = nextToken
		if next == "" {
			fmt.Fprintln(os.Stderr, "")
			return vv, nil
		}
	}
}

// Retrieve snippet of the specified playlist
func (c *client) snippet(playlistId string, pageToken string) (snippet []string, next string, err error) {
	call := c.y.PlaylistItems.List("snippet")
	call = call.PlaylistId(playlistId)
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	r, err := call.Do()
	if err != nil {
		return nil, "", err
	}
	var vv []string
	for _, v := range r.Items {
		vv = append(vv, v.Snippet.ResourceId.VideoId)
	}
	return vv, r.NextPageToken, nil
}

func (c *client) publish(ctx context.Context, name string, vids []string) error {
	p, err := c.y.Playlists.Insert("snippet,status", &youtube.Playlist{
		Snippet: &youtube.PlaylistSnippet{
			Title: name,
		},
	}).Do()
	if err != nil {
		return fmt.Errorf("couldn't create playlist: %v", err)
	}
	pid := p.Id

	// TODO wtf?
	_ = pid

	for _, v := range vids {
		_, err := c.y.PlaylistItems.Insert("snippet,status", &youtube.PlaylistItem{
			Id: v,
		}).Do()
		if err != nil {
			return fmt.Errorf("couldn't insert into playlist: %v", err)
		}
	}
	return nil
}

func split(ss []string, d int) ([][]string, error) {
	vv := make([][]string, d)
	for _, s := range ss {
		r, err := ran(d)
		if err != nil {
			return nil, err
		}
		vv[r] = append(vv[r], s)
	}
	return vv, nil
}

func ran(d int) (int, error) {
	b, err := rand.Int(rand.Reader, big.NewInt(int64(d)))
	if err != nil {
		return 0, err
	}
	return int(b.Int64()), nil
}

func randomString() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func makeAuthRequest(clientID string) error {
	authURL := "https://accounts.google.com/o/oauth2/v2/auth"

	codeChallenge, err := randomString()
	if err != nil {
		return err
	}
	state, err := randomString()
	if err != nil {
		return err
	}

	v := make(url.Values)
	v.Set("client_id", clientID)
	v.Set("redirect_uri", "http://127.0.0.1:8080")
	v.Set("response_type", "code")
	v.Set("scope", "https://www.googleapis.com/auth/youtube https://www.googleapis.com/auth/youtube.readonly")
	v.Set("code_challenge", codeChallenge)
	v.Set("state", state)

	u := authURL + "?" + v.Encode()

	res, err := http.Get(u)
	if err != nil {
		return err
	}

	// Wait for something?

	return nil
}
