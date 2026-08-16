// Package hls содержит чистые функции переписывания HLS-плейлистов (§4.3 дизайна эпика):
// URI вариантов мастер-плейлиста и сегментов медиаплейлиста заменяются на подписанные URL,
// строки, начинающиеся с '#', и переводы строк не изменяются.
package hls

import (
	"fmt"
	"strings"
)

// RewriteMaster переписывает мастер-плейлист HLS: каждая не-'#' строка (URI варианта, например
// "720p/playlist.m3u8") заменяется на результат variantURL(profile), где profile — первый
// сегмент пути URI ("720p"). Строки, начинающиеся с '#' (в том числе #EXT-X-MEDIA с атрибутом
// URI="...") и пустые строки не изменяются. Порядок строк и переводы строк (включая \r\n)
// сохраняются.
func RewriteMaster(src []byte, variantURL func(profile string) string) ([]byte, error) {
	return rewriteLines(src, func(uri string) (string, error) {
		return variantURL(firstPathSegment(uri)), nil
	})
}

// RewriteMedia переписывает медиаплейлист HLS: каждая не-'#' строка (имя сегмента, например
// "seg_00001.ts") заменяется на результат segmentURL(name). Ошибка, возвращённая segmentURL,
// прерывает переписывание и возвращается вызывающему.
func RewriteMedia(src []byte, segmentURL func(name string) (string, error)) ([]byte, error) {
	return rewriteLines(src, segmentURL)
}

// Profiles извлекает имена профилей из мастер-плейлиста в порядке их появления (для
// dto.GetVideoResponse.Profiles). Профиль — первый сегмент пути URI варианта.
func Profiles(master []byte) []string {
	var profiles []string

	seen := make(map[string]struct{})

	for raw := range strings.SplitSeq(string(master), "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		profile := firstPathSegment(line)
		if _, ok := seen[profile]; ok {
			continue
		}

		seen[profile] = struct{}{}
		profiles = append(profiles, profile)
	}

	return profiles
}

// rewriteLines построчно переписывает плейлист: строки, начинающиеся с '#', и пустые строки
// оставляет без изменений, остальные — заменяет результатом rewrite. Разбиение по '\n' с
// посегментным сохранением завершающего '\r' поддерживает как unix-, так и dos-переводы строк.
func rewriteLines(src []byte, rewrite func(uri string) (string, error)) ([]byte, error) {
	rawLines := strings.Split(string(src), "\n")

	for i, raw := range rawLines {
		line := raw
		carriageReturn := ""

		if trimmed, ok := strings.CutSuffix(line, "\r"); ok {
			line = trimmed
			carriageReturn = "\r"
		}

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		rewritten, err := rewrite(line)
		if err != nil {
			return nil, fmt.Errorf("rewrite line %d: %w", i+1, err)
		}

		rawLines[i] = rewritten + carriageReturn
	}

	return []byte(strings.Join(rawLines, "\n")), nil
}

// firstPathSegment возвращает первый сегмент пути URI до '/' — имя профиля для строк вида
// "720p/playlist.m3u8". Если разделителя нет, возвращает строку целиком.
func firstPathSegment(uri string) string {
	if segment, _, ok := strings.Cut(uri, "/"); ok {
		return segment
	}

	return uri
}
