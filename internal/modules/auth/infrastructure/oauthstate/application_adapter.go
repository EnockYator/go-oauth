package oauthstate

import (
	authapp "github.com/EnockYator/go-oauth/internal/modules/auth/application"
	"time"
)

type ApplicationCodec struct{ codec *Codec }

func NewApplicationCodec(codec *Codec) *ApplicationCodec { return &ApplicationCodec{codec: codec} }
func (c *ApplicationCodec) Encode(p authapp.StatePayload) (string, error) {
	return c.codec.Encode(Payload{State: p.State, CodeVerifier: p.CodeVerifier, Provider: p.Provider, ReturnTo: p.ReturnTo})
}
func (c *ApplicationCodec) Decode(encoded string) (*authapp.StatePayload, error) {
	p, err := c.codec.Decode(encoded)
	if err != nil {
		return nil, err
	}
	return &authapp.StatePayload{State: p.State, CodeVerifier: p.CodeVerifier, Provider: p.Provider, ReturnTo: p.ReturnTo}, nil
}
func (c *ApplicationCodec) TTL() time.Duration { return c.codec.TTL() }
