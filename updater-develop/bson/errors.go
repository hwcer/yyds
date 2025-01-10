package bson

import "errors"

var (
	ErrorNotChange          = errors.New("ErrorNotChange")
	ErrorNoPrimaryKey       = errors.New("NoPrimaryKey")
	ErrorDocumentExist      = errors.New("ErrorDocumentExist")
	ErrorElementExist       = errors.New("ErrorElementExist")
	ErrorElementNotExist    = errors.New("ErrorElementNotExist")
	ErrorElementNotSlice    = errors.New("ErrorElementNotSlice")
	ErrorElementNotDocument = errors.New("ErrorElementNotDocument")
	ErrorElementTypeIllegal = errors.New("ErrorElementTypeIllegal")
	ErrorSliceIndexIllegal  = errors.New("ErrorSliceIndexIllegal")
	ErrorDocumentNotExist   = errors.New("ErrorDocumentNotExist")

	ErrorNotValidNumber = errors.New("not a valid number")
)
