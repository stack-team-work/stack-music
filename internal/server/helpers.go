package server

import (
	"encoding/json"

	"stack-music/internal/model"
)

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func queueToResp(songs []*model.Song, conv func(*model.Song) *songResp) []*songResp {
	out := make([]*songResp, 0, len(songs))
	for _, so := range songs {
		out = append(out, conv(so))
	}
	return out
}
