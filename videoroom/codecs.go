package videoroom

import (
	"fmt"
	"strings"

	"github.com/pion/rtp/v2"
	"github.com/pion/rtp/v2/codecs"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"
)

type rtpamp struct {
	pt        uint8
	name      string
	clockrate int
	channels  int
}

func newrtpmap(value string) *rtpamp {
	m := &rtpamp{}
	value = strings.ReplaceAll(value, "/", " ")
	fmt.Sscanf(value, "%d %s %d %d", &m.pt, &m.name, &m.clockrate, &m.channels)
	return m
}

func (rm *rtpamp) getPayloader() rtp.Payloader {

	switch strings.ToLower(rm.name) {
	case "pcma":

		return &codecs.G711Payloader{}
	case "opus":
		return &codecs.OpusPayloader{}
	case "g722":
		return &codecs.G722Payloader{}
	case "vp8":
		return &codecs.VP8Payloader{}
	case "vp9":
		return &codecs.VP9Payloader{}
	case "h264":
		return &codecs.H264Payloader{}
	case "rsfec":
		return &codecs.OpusPayloader{}
	case "flexfec-03":
		return &codecs.G711Payloader{}
	default:
		return &codecs.G711Payloader{}
	}
}

func initAPI(remoteSDP string) *webrtc.API {
	sd := sdp.SessionDescription{}
	err := sd.Unmarshal([]byte(remoteSDP))
	if err != nil {
		return nil
	}

	m := webrtc.MediaEngine{}

	m.RegisterDefaultCodecs()

	setting := webrtc.SettingEngine{}
	setting.SetEphemeralUDPPortRange(20000, 40000)

	return webrtc.NewAPI(webrtc.WithMediaEngine(&m), webrtc.WithSettingEngine(setting))
}
