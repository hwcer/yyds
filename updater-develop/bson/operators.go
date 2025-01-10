package bson

import "go.mongodb.org/mongo-driver/bson/bsontype"

func (ele *Element) Inc(v interface{}) (r interface{}, err error) {
	if !IsNumber(v) {
		return 0, ErrorNotValidNumber
	}
	if ele.IsNil() {
		return v, ele.Marshal(v)
	}
	switch ele.val.Type {
	case bsontype.Int32:
		r = ele.GetInt32() + ParseInt32(v)
		err = ele.Marshal(r)
	case bsontype.Int64:
		r = ele.GetInt64() + ParseInt64(v)
		err = ele.Marshal(r)
	case bsontype.Double:
		r = ele.GetFloat() + ParseDouble(v)
		err = ele.Marshal(r)
	default:
		err = ErrorNotValidNumber
	}
	return
}

func (ele *Element) Min(v interface{}) (r interface{}, err error) {
	if !IsNumber(v) {
		return 0, ErrorNotValidNumber
	}
	if ele.IsNil() {
		return v, ele.Marshal(v)
	}
	err = ErrorNotChange
	switch ele.val.Type {
	case bsontype.Int32:
		v1, v2 := ParseInt32(v), ele.GetInt32()
		if v1 < v2 {
			r = v1
			err = ele.Marshal(r)
		} else {
			r = v2
		}
	case bsontype.Int64:
		v1, v2 := ParseInt64(v), ele.GetInt64()
		if v1 < v2 {
			r = v1
			err = ele.Marshal(r)
		} else {
			r = v2
		}
	case bsontype.Double:
		v1, v2 := ParseDouble(v), ele.GetFloat()
		if v1 < v2 {
			r = v1
			err = ele.Marshal(r)
		} else {
			r = v2
		}
	default:
		err = ErrorNotValidNumber
	}
	return
}

func (ele *Element) Max(v interface{}) (r interface{}, err error) {
	if !IsNumber(v) {
		return 0, ErrorNotValidNumber
	}
	if ele.IsNil() {
		return v, ele.Marshal(v)
	}
	err = ErrorNotChange
	switch ele.val.Type {
	case bsontype.Int32:
		v1, v2 := ParseInt32(v), ele.GetInt32()
		if v1 > v2 {
			r = v1
			err = ele.Marshal(r)
		} else {
			r = v2
		}
	case bsontype.Int64:
		v1, v2 := ParseInt64(v), ele.GetInt64()
		if v1 > v2 {
			r = v1
			err = ele.Marshal(r)
		} else {
			r = v2
		}
	case bsontype.Double:
		v1, v2 := ParseDouble(v), ele.GetFloat()
		if v1 > v2 {
			r = v1
			err = ele.Marshal(r)
		} else {
			r = v2
		}
	default:
		err = ErrorNotValidNumber
	}
	return
}

func (ele *Element) Mul(v interface{}) (r interface{}, err error) {
	if !IsNumber(v) {
		return 0, ErrorNotValidNumber
	}
	if ele.IsNil() {
		return 0, nil
	}
	switch ele.val.Type {
	case bsontype.Int32:
		r = ele.GetInt32() * ParseInt32(v)
		err = ele.Marshal(r)
	case bsontype.Int64:
		r = ele.GetInt64() * ParseInt64(v)
		err = ele.Marshal(r)
	case bsontype.Double:
		r = ele.GetFloat() * ParseDouble(v)
		err = ele.Marshal(r)
	default:
		err = ErrorNotValidNumber
	}
	return
}
