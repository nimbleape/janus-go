package videoroom

import (
	"context"
	"fmt"

	"github.com/nimbleape/janus-go/jwsapi"
	"github.com/nimbleape/janus-go/jwsapi/jplugin/jvideoroom"
	"github.com/nimbleape/janus-go/logging"
	"github.com/pion/webrtc/v3"
	"github.com/pkg/errors"
)

//Subscriber a subscriber object.
type Subscriber struct {
	BaseSession
	jSub         *jvideoroom.Subscriber
	transceivers []*webrtc.RTPTransceiver //using for send rtcp to remote, RR

	onAudioTrack func(context.Context, *webrtc.PeerConnection, *webrtc.TrackRemote, *webrtc.RTPReceiver)
	onVideoTrack func(context.Context, *webrtc.PeerConnection, *webrtc.TrackRemote, *webrtc.RTPReceiver)
}

//SubscriberOption option for Subscriber
type SubscriberOption func(*Subscriber)

//WithSubscriberAudioTrack using to setting audio track callback
func WithSubscriberAudioTrack(callback func(context.Context, *webrtc.PeerConnection, *webrtc.TrackRemote, *webrtc.RTPReceiver)) SubscriberOption {
	return func(s *Subscriber) {
		s.onAudioTrack = callback
	}
}

//WithSubscriberVideoTrack using to setting audio track callback
func WithSubscriberVideoTrack(callback func(context.Context, *webrtc.PeerConnection, *webrtc.TrackRemote, *webrtc.RTPReceiver)) SubscriberOption {
	return func(s *Subscriber) {
		s.onVideoTrack = callback
	}
}

//WithSubscriberConfigure set webrtc configure
func WithSubscriberConfigure(configure webrtc.Configuration) SubscriberOption {
	return func(s *Subscriber) {
		s.configure = configure
	}
}

//NewSubscriber new subscriber
func NewSubscriber(ctx context.Context, api *webrtc.API, h *jwsapi.Handle, room string, feed string, configuration webrtc.Configuration) *Subscriber {
	s := &Subscriber{
		BaseSession: BaseSession{
			ctx:    ctx,
			api:    api,
			handle: h,
			configure: configuration,
			remoteCandidates: make(chan jwsapi.Message, 8),
		},
		jSub: jvideoroom.NewSubscriber(ctx, h, room, feed),
	}

	h.SetCallback(jwsapi.WithHandleTrickle(s.onTrickle))
	h.SetCallback(jwsapi.WithHandleHangup(s.onHangup))
	// h.SetCallback(jwsapi.WithHandleWebrtcup(s.onWebrtcup))

	return s
}

//Object return jvideoroom.Subscriber
func (s *Subscriber) Object() *jvideoroom.Subscriber {
	return s.jSub
}

//ID return id for this subscriber
func (s *Subscriber) ID() string {
	return fmt.Sprintf("[%s.Feed.%s", s.jSub.Room(), s.jSub.Feed())
}

//SetOption set option, for callback
func (s *Subscriber) SetOption(opts ...SubscriberOption) *Subscriber {
	for _, opt := range opts {
		opt(s)
	}
	return s
}

//Start start pull stream from janus
//audio,video default is true, data default is false
//optional or default param can use jwsapi.WithMessageOption to setting
//other params see  https://jwsapi.conf.meetecho.com/docs/videoroom.html VideoRoom Subscribers join
//jwsapi.WithMessageOption("video",false) to ignore video stream
//janus-gateway must open ice-lite=true
func (s *Subscriber) Start(opts ...jwsapi.MessageOption) error {

	offer, err := s.jSub.Join(opts...)
	if err != nil {
		return errors.Wrap(err, "join")
	}

	api := initAPI(offer)
	if api != nil {
		s.api = api
	}

	pc, err := s.api.NewPeerConnection(s.configure)
	if err != nil {
		return errors.Wrap(err, "NewPeerConnection")
	}
	s.pc = pc

	pc.OnTrack(s.onTrack)
	pc.OnConnectionStateChange(s.onPeerConnectionState)
	pc.OnICECandidate(s.onICECandidate)
	pc.OnICEConnectionStateChange(s.onICEConnectionStateChange)

	err = pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offer,
	})
	if err != nil {
		pc.Close()
		return errors.Wrap(err, "pc.SetRemoteDescription")
	}

	recvOnly := webrtc.RtpTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}
	at, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, recvOnly)

	if err != nil {
		pc.Close()
		return errors.Wrap(err, "AddTransceiver(Audio)")
	}
	s.transceivers = append(s.transceivers, at)

	vt, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, recvOnly)
	if err != nil {
		pc.Close()
		return errors.Wrap(err, "AddTransceiver(Video)")
	}
	s.transceivers = append(s.transceivers, vt)

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return errors.Wrap(err, "pc.CreateOffer")
	}

	pc.SetLocalDescription(answer)

	err = s.jSub.Start(answer.SDP, true)
	if err != nil {
		pc.Close()
		return errors.Wrap(err, "jvideoroom.Subscriber.Start")
	}

	go s.doRemoteCandidate(s.remoteCandidates)
	return nil
}

//Leave leave pull stream
func (s *Subscriber) Leave() error {
	if s.pc != nil {
		s.pc.Close()
	}
	return s.jSub.Leave()

}

func (s *Subscriber) onHangup(msg jwsapi.Message) {
	s.pc.Close()
}
func (s *Subscriber) onWebrtcup(msg jwsapi.Message) {
	for _, tr := range s.transceivers {
		go s.startRTPTransceiver(tr)
	}
}

func (s *Subscriber) onICEConnectionStateChange(state webrtc.ICEConnectionState) {
	logging.Infof("ICEConnectionState %s", state.String())
}
func (s *Subscriber) onPeerConnectionState(state webrtc.PeerConnectionState) {
	logging.Infof("PeerConnectionState %s", state.String())
}

// func (s *Subscriber) onICECandidate(candidate *webrtc.ICECandidate) {
// 	log.Printf("local ice candidate from local: %v\n", candidate)
// }

func (s *Subscriber) onTrack(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	logging.Infof("%s onTrack %s SSRC %d PT %d", s.ID(), track.Kind().String(), track.SSRC(), track.PayloadType())

	go s.startReceiver(receiver)

	switch track.Kind() {
	case webrtc.RTPCodecTypeAudio:
		if s.onAudioTrack != nil {
			s.onAudioTrack(s.ctx, s.pc, track, receiver)
			return
		}
	case webrtc.RTPCodecTypeVideo:
		if s.onVideoTrack != nil {
			s.onVideoTrack(s.ctx, s.pc, track, receiver)
			return
		}
	}

	//no callback for user
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			//read rtp form track...
			_, _, err := track.ReadRTP()
			if err != nil {
				return
			}
		}
	}
}

func (s *Subscriber) startRTPTransceiver(tr *webrtc.RTPTransceiver) {

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			if sender := tr.Sender(); sender != nil {
				_, _, err := sender.ReadRTCP()
				if err != nil {
					return
				}
			}
		}
	}
}

func (s *Subscriber) startReceiver(receiver *webrtc.RTPReceiver) {

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			_, _, err := receiver.ReadRTCP()
			if err != nil {
				return
			}
		}
	}
}
