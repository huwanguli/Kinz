package kiface

type IDecoder interface {
	IInterceptor
	GetLengthField() *LengthField
}
