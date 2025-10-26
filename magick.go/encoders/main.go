package encoders

type Encoder map[string]func(input string, settings map[string]string) error

func (e *Encoder) registerEncoder(name string, handler func(input string, settings map[string]string) error) {
	(*e)[name] = handler
}

var encoders = make(Encoder)

func init() {
	encoders.registerEncoder("jpg", HandleJPEG)
	encoders.registerEncoder("webp", HandleWEBP)
	encoders.registerEncoder("avif", HandleAVIF)
}
