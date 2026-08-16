package hls_test

import (
	"errors"
	"os"
	"strconv"
	"testing"
	"vilib-api/internal/hls"

	"github.com/stretchr/testify/require"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)

	return data
}

func TestRewriteMaster_ReplacesVariantURIsWithProfileURL(t *testing.T) {
	t.Parallel()

	src := readTestdata(t, "master.m3u8")
	want := readTestdata(t, "master_rewritten.m3u8")

	got, err := hls.RewriteMaster(src, func(profile string) string {
		return profile + "/playlist.m3u8?token=test-token"
	})

	require.NoError(t, err)
	require.Equal(t, string(want), string(got))
}

func TestRewriteMaster_PreservesCRLFLineEndings(t *testing.T) {
	t.Parallel()

	src := readTestdata(t, "master_crlf.m3u8")

	got, err := hls.RewriteMaster(src, func(profile string) string {
		return profile + "/playlist.m3u8?token=x"
	})

	require.NoError(t, err)
	require.Equal(
		t,
		"#EXTM3U\r\n#EXT-X-STREAM-INF:BANDWIDTH=800000\r\n360p/playlist.m3u8?token=x\r\n",
		string(got),
	)
}

func TestRewriteMaster_DoesNotTouchCommentLines(t *testing.T) {
	t.Parallel()

	src := []byte(
		"#EXTM3U\n" +
			"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"aac\",NAME=\"stereo\",URI=\"audio/playlist.m3u8\"\n" +
			"#EXT-X-STREAM-INF:BANDWIDTH=800000\n" +
			"360p/playlist.m3u8\n",
	)

	got, err := hls.RewriteMaster(src, func(profile string) string {
		return profile + "/playlist.m3u8?token=x"
	})

	require.NoError(t, err)
	require.Contains(t, string(got), "URI=\"audio/playlist.m3u8\"")
	require.Contains(t, string(got), "360p/playlist.m3u8?token=x")
}

func TestRewriteMedia_ReplacesSegmentNamesWithSignedURLs(t *testing.T) {
	t.Parallel()

	src := readTestdata(t, "media.m3u8")
	want := readTestdata(t, "media_rewritten.m3u8")

	call := 0
	got, err := hls.RewriteMedia(src, func(name string) (string, error) {
		call++
		return "https://s3.example.com/videos/hls/720p/" + name + "?sig=" + strconv.Itoa(call), nil
	})

	require.NoError(t, err)
	require.Equal(t, string(want), string(got))
	require.Equal(t, 3, call)
}

func TestRewriteMedia_PropagatesSegmentURLError(t *testing.T) {
	t.Parallel()

	src := readTestdata(t, "media.m3u8")
	segmentErr := errors.New("presign failed")

	_, err := hls.RewriteMedia(src, func(_ string) (string, error) {
		return "", segmentErr
	})

	require.ErrorIs(t, err, segmentErr)
}

func TestRewriteMedia_StopsAtFirstFailingSegment(t *testing.T) {
	t.Parallel()

	src := readTestdata(t, "media.m3u8")
	segmentErr := errors.New("presign failed")

	call := 0
	_, err := hls.RewriteMedia(src, func(_ string) (string, error) {
		call++
		if call == 2 {
			return "", segmentErr
		}
		return "ok", nil
	})

	require.ErrorIs(t, err, segmentErr)
	require.Equal(t, 2, call)
}

func TestProfiles_ExtractsProfileNamesInOrder(t *testing.T) {
	t.Parallel()

	src := readTestdata(t, "master.m3u8")

	got := hls.Profiles(src)

	require.Equal(t, []string{"360p", "720p", "1080p"}, got)
}

func TestProfiles_EmptyMasterReturnsNoProfiles(t *testing.T) {
	t.Parallel()

	got := hls.Profiles([]byte("#EXTM3U\n"))

	require.Empty(t, got)
}
