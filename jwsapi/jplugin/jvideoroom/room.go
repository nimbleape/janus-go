package jvideoroom

import "github.com/nimbleape/janus-go/jwsapi"

//Room room info
type Room struct {
	jwsapi.Message
}

//ID room id
func (r *Room) ID() string {
	room, _ := r.String("room")
	return room
}

//Participant part
type Participant struct {
	jwsapi.Message
}

//ID return publisher id
func (r *Participant) ID() string {
	id, _ := r.String("id")
	return id
}

//Display get display
func (r *Participant) Display() string {
	display, _ := r.String("display")
	return display
}

//AudioCodec audio codec for this publisher
func (r *Participant) AudioCodec() string {
	codec, _ := r.String("audio_codec")
	return codec
}

//VideoCodec video codec for this publisher
func (r *Participant) VideoCodec() string {
	codec, _ := r.String("video_codec")
	return codec
}

//Simulcast only for VP8 and H.264
func (r *Participant) Simulcast() bool {
	return r.Bool("simulcast")
}
