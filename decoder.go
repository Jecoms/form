package form

import (
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strconv"
	"time"
)

const (
	errArraySize           = "Array size of '%d' is larger than the maximum currently set on the decoder of '%d'. To increase this limit please see, SetMaxArraySize(size uint)"
	errMissingStartBracket = "Invalid formatting for key '%s' missing '[' bracket"
	errMissingEndBracket   = "Invalid formatting for key '%s' missing ']' bracket"
)

type decoder struct {
	d         *Decoder
	errs      DecodeErrors
	dm        dataMap
	aliasMap  map[string]*recursiveData
	values    url.Values
	maxKeyLen int
	namespace []byte
}

func (d *decoder) setError(namespace []byte, err error) {
	if d.errs == nil {
		d.errs = make(DecodeErrors)
	}
	d.errs[string(namespace)] = err
}

func (d *decoder) findAlias(ns string) *recursiveData {
	if d.aliasMap != nil {
		return d.aliasMap[ns]
	}
	return nil
}

func (d *decoder) parseMapData() {
	// already parsed
	if len(d.dm) > 0 {
		return
	}

	d.maxKeyLen = 0
	d.dm = d.dm[0:0]

	if d.aliasMap == nil {
		d.aliasMap = make(map[string]*recursiveData)
	} else {
		for k := range d.aliasMap {
			delete(d.aliasMap, k)
		}
	}

	var i int
	var idx int
	var l int
	var insideBracket bool
	var rd *recursiveData
	var isNum bool

	for k := range d.values {

		if len(k) > d.maxKeyLen {
			d.maxKeyLen = len(k)
		}

		for i = 0; i < len(k); i++ {

			switch k[i] {
			case '[':
				idx = i
				insideBracket = true
				isNum = true
			case ']':

				if !insideBracket {
					log.Panicf(errMissingStartBracket, k)
				}

				if rd = d.findAlias(k[:idx]); rd == nil {

					l = len(d.dm) + 1

					if l > cap(d.dm) {
						dm := make(dataMap, l)
						copy(dm, d.dm)
						rd = new(recursiveData)
						dm[len(d.dm)] = rd
						d.dm = dm
					} else {
						l = len(d.dm)
						d.dm = d.dm[:l+1]
						rd = d.dm[l]
						rd.sliceLen = 0
						rd.keys = rd.keys[0:0]
					}

					rd.alias = k[:idx]
					d.aliasMap[rd.alias] = rd
				}

				// is map + key
				ke := key{
					ivalue:      -1,
					value:       k[idx+1 : i],
					searchValue: k[idx : i+1],
				}

				// is key is number, most likely array key, keep track of just in case an array/slice.
				if isNum {

					// no need to check for error, it will always pass
					// as we have done the checking to ensure
					// the value is a number ahead of time.
					var err error
					ke.ivalue, err = strconv.Atoi(ke.value)
					if err != nil {
						ke.ivalue = -1
					}

					if ke.ivalue > rd.sliceLen {
						rd.sliceLen = ke.ivalue

					}
				}

				rd.keys = append(rd.keys, ke)

				insideBracket = false
			default:
				// checking if not a number, 0-9 is 48-57 in byte, see for yourself fmt.Println('0', '1', '2', '3', '4', '5', '6', '7', '8', '9')
				if insideBracket && (k[i] > 57 || k[i] < 48) {
					isNum = false
				}
			}
		}

		// if still inside bracket, that means no ending bracket was ever specified
		if insideBracket {
			log.Panicf(errMissingEndBracket, k)
		}
	}
}

func (d *decoder) traverseStruct(v reflect.Value, typ reflect.Type, namespace []byte) (set bool) {

	l := len(namespace)
	first := l == 0

	// anonymous structs will still work for caching as the whole definition is stored
	// including tags
	s, ok := d.d.structCache.Get(typ)
	if !ok {
		s = d.d.structCache.parseStruct(d.d.mode, v, typ, d.d.tagName)
	}

	for _, f := range s.fields {
		namespace = namespace[:l]

		if f.isAnonymous {
			if d.setFieldByType(v.Field(f.idx), namespace, 0) {
				set = true
			}
		}

		if first {
			namespace = append(namespace, f.name...)
		} else {
			namespace = append(namespace, d.d.namespacePrefix...)
			namespace = append(namespace, f.name...)
			namespace = append(namespace, d.d.namespaceSuffix...)
		}

		if d.setFieldByType(v.Field(f.idx), namespace, 0) {
			set = true
		}
	}

	return
}

// parseAndSetUint parses a string value as unsigned integer and sets it on the reflect.Value
func (d *decoder) parseAndSetUint(v reflect.Value, value string, bitSize int, namespace []byte, ns string) error {
	if len(value) == 0 {
		return nil
	}
	u64, err := strconv.ParseUint(value, 10, bitSize)
	if err != nil {
		return fmt.Errorf("Invalid Unsigned Integer Value '%s' Type '%v' Namespace '%s'", value, v.Type(), ns)
	}
	v.SetUint(u64)
	return nil
}

// parseAndSetInt parses a string value as signed integer and sets it on the reflect.Value
func (d *decoder) parseAndSetInt(v reflect.Value, value string, bitSize int, namespace []byte, ns string) error {
	if len(value) == 0 {
		return nil
	}
	i64, err := strconv.ParseInt(value, 10, bitSize)
	if err != nil {
		return fmt.Errorf("Invalid Integer Value '%s' Type '%v' Namespace '%s'", value, v.Type(), ns)
	}
	v.SetInt(i64)
	return nil
}

// parseAndSetFloat parses a string value as float and sets it on the reflect.Value
func (d *decoder) parseAndSetFloat(v reflect.Value, value string, bitSize int, namespace []byte, ns string) error {
	if len(value) == 0 {
		return nil
	}
	f, err := strconv.ParseFloat(value, bitSize)
	if err != nil {
		return fmt.Errorf("Invalid Float Value '%s' Type '%v' Namespace '%s'", value, v.Type(), ns)
	}
	v.SetFloat(f)
	return nil
}

// parseAndSetUintKey is similar to parseAndSetUint but for map keys
func (d *decoder) parseAndSetUintKey(v reflect.Value, key string, bitSize int, ns string) error {
	u64, err := strconv.ParseUint(key, 10, bitSize)
	if err != nil {
		return fmt.Errorf("Invalid Unsigned Integer Value '%s' Type '%v' Namespace '%s'", key, v.Type(), ns)
	}
	v.SetUint(u64)
	return nil
}

// parseAndSetIntKey is similar to parseAndSetInt but for map keys
func (d *decoder) parseAndSetIntKey(v reflect.Value, key string, bitSize int, ns string) error {
	i64, err := strconv.ParseInt(key, 10, bitSize)
	if err != nil {
		return fmt.Errorf("Invalid Integer Value '%s' Type '%v' Namespace '%s'", key, v.Type(), ns)
	}
	v.SetInt(i64)
	return nil
}

// parseAndSetFloatKey is similar to parseAndSetFloat but for map keys
func (d *decoder) parseAndSetFloatKey(v reflect.Value, key string, bitSize int, ns string) error {
	f, err := strconv.ParseFloat(key, bitSize)
	if err != nil {
		return fmt.Errorf("Invalid Float Value '%s' Type '%v' Namespace '%s'", key, v.Type(), ns)
	}
	v.SetFloat(f)
	return nil
}

// parseAndSetBool parses a string value as boolean and sets it on the reflect.Value
func (d *decoder) parseAndSetBool(v reflect.Value, value string, namespace []byte, ns string) error {
	b, err := parseBool(value)
	if err != nil {
		return fmt.Errorf("Invalid Boolean Value '%s' Type '%v' Namespace '%s'", value, v.Type(), ns)
	}
	v.SetBool(b)
	return nil
}

// parseAndSetBoolKey is similar to parseAndSetBool but for map keys
func (d *decoder) parseAndSetBoolKey(v reflect.Value, key string, ns string) error {
	b, err := parseBool(key)
	if err != nil {
		return fmt.Errorf("Invalid Boolean Value '%s' Type '%v' Namespace '%s'", key, v.Type(), ns)
	}
	v.SetBool(b)
	return nil
}

// setPrimitiveMapKey handles setting primitive types for map keys (uint, int, float, bool)
func (d *decoder) setPrimitiveMapKey(v reflect.Value, key string, kind reflect.Kind, ns string) error {
	switch kind {
	case reflect.Uint, reflect.Uint64:
		return d.parseAndSetUintKey(v, key, 64, ns)
	case reflect.Uint8:
		return d.parseAndSetUintKey(v, key, 8, ns)
	case reflect.Uint16:
		return d.parseAndSetUintKey(v, key, 16, ns)
	case reflect.Uint32:
		return d.parseAndSetUintKey(v, key, 32, ns)
	case reflect.Int, reflect.Int64:
		return d.parseAndSetIntKey(v, key, 64, ns)
	case reflect.Int8:
		return d.parseAndSetIntKey(v, key, 8, ns)
	case reflect.Int16:
		return d.parseAndSetIntKey(v, key, 16, ns)
	case reflect.Int32:
		return d.parseAndSetIntKey(v, key, 32, ns)
	case reflect.Float32:
		return d.parseAndSetFloatKey(v, key, 32, ns)
	case reflect.Float64:
		return d.parseAndSetFloatKey(v, key, 64, ns)
	case reflect.Bool:
		return d.parseAndSetBoolKey(v, key, ns)
	}
	return nil
}

// setPrimitiveValue handles setting primitive types (uint, int, float, bool)
func (d *decoder) setPrimitiveValue(v reflect.Value, arr []string, idx int, kind reflect.Kind, namespace []byte, ns string) (set bool, err error) {
	if idx >= len(arr) {
		return false, nil
	}

	switch kind {
	case reflect.Uint, reflect.Uint64:
		err = d.parseAndSetUint(v, arr[idx], 64, namespace, ns)
	case reflect.Uint8:
		err = d.parseAndSetUint(v, arr[idx], 8, namespace, ns)
	case reflect.Uint16:
		err = d.parseAndSetUint(v, arr[idx], 16, namespace, ns)
	case reflect.Uint32:
		err = d.parseAndSetUint(v, arr[idx], 32, namespace, ns)
	case reflect.Int, reflect.Int64:
		err = d.parseAndSetInt(v, arr[idx], 64, namespace, ns)
	case reflect.Int8:
		err = d.parseAndSetInt(v, arr[idx], 8, namespace, ns)
	case reflect.Int16:
		err = d.parseAndSetInt(v, arr[idx], 16, namespace, ns)
	case reflect.Int32:
		err = d.parseAndSetInt(v, arr[idx], 32, namespace, ns)
	case reflect.Float32:
		err = d.parseAndSetFloat(v, arr[idx], 32, namespace, ns)
	case reflect.Float64:
		err = d.parseAndSetFloat(v, arr[idx], 64, namespace, ns)
	case reflect.Bool:
		err = d.parseAndSetBool(v, arr[idx], namespace, ns)
	}

	if err != nil {
		return false, err
	}
	return true, nil
}

// setSlice handles decoding of slice types
func (d *decoder) setSlice(v reflect.Value, arr []string, namespace []byte, ns string) (set bool) {
	// slice elements could be mixed eg. number and non-numbers Value[0]=[]string{"10"} and Value=[]string{"10","20"}
	if len(arr) > 0 {
		var varr reflect.Value
		var ol int
		l := len(arr)

		if v.IsNil() {
			varr = reflect.MakeSlice(v.Type(), len(arr), len(arr))
		} else {
			ol = v.Len()
			l += ol

			if v.Cap() <= l {
				varr = reflect.MakeSlice(v.Type(), l, l)
			} else {
				// preserve predefined capacity, possibly for reuse after decoding
				varr = reflect.MakeSlice(v.Type(), l, v.Cap())
			}
			reflect.Copy(varr, v)
		}

		for i := ol; i < l; i++ {
			newVal := reflect.New(v.Type().Elem()).Elem()
			if d.setFieldByType(newVal, namespace, i-ol) {
				set = true
				varr.Index(i).Set(newVal)
			}
		}
		v.Set(varr)
	}
	return
}

// setSliceWithIndexes handles decoding of slice types with explicit indexes
func (d *decoder) setSliceWithIndexes(v reflect.Value, rd *recursiveData, namespace []byte) (set bool) {
	var varr reflect.Value
	var kv key

	sl := rd.sliceLen + 1

	// checking below for maxArraySize, but if array exists and already
	// has sufficient capacity allocated then we do not check as the code
	// obviously allows a capacity greater than the maxArraySize.

	if v.IsNil() {
		if sl > d.d.maxArraySize {
			d.setError(namespace, fmt.Errorf(errArraySize, sl, d.d.maxArraySize))
			return
		}
		varr = reflect.MakeSlice(v.Type(), sl, sl)
	} else if v.Len() < sl {
		if v.Cap() <= sl {
			if sl > d.d.maxArraySize {
				d.setError(namespace, fmt.Errorf(errArraySize, sl, d.d.maxArraySize))
				return
			}
			varr = reflect.MakeSlice(v.Type(), sl, sl)
		} else {
			varr = reflect.MakeSlice(v.Type(), sl, v.Cap())
		}
		reflect.Copy(varr, v)
	} else {
		varr = v
	}

	for i := 0; i < len(rd.keys); i++ {
		kv = rd.keys[i]
		newVal := reflect.New(varr.Type().Elem()).Elem()

		if kv.ivalue == -1 {
			d.setError(namespace, fmt.Errorf("invalid slice index '%s'", kv.value))
			continue
		}

		if d.setFieldByType(newVal, append(namespace, kv.searchValue...), 0) {
			set = true
			varr.Index(kv.ivalue).Set(newVal)
		}
	}

	if set {
		v.Set(varr)
	}
	return
}

// setArray handles decoding of array types
func (d *decoder) setArray(v reflect.Value, arr []string, namespace []byte) (set bool) {
	// array elements could be mixed eg. number and non-numbers Value[0]=[]string{"10"} and Value=[]string{"10","20"}
	if len(arr) > 0 {
		var varr reflect.Value
		l := len(arr)
		overCapacity := v.Len() < l
		if overCapacity {
			// more values than array capacity, ignore values over capacity as it's possible some would just want
			// to grab the first x number of elements; in the future strict mode logic should return an error
			fmt.Println("warning number of post form array values is larger than array capacity, ignoring overflow values")
		}
		varr = reflect.Indirect(reflect.New(reflect.ArrayOf(v.Len(), v.Type().Elem())))
		reflect.Copy(varr, v)

		if v.Len() < len(arr) {
			l = v.Len()
		}
		for i := 0; i < l; i++ {
			newVal := reflect.New(v.Type().Elem()).Elem()
			if d.setFieldByType(newVal, namespace, i) {
				set = true
				varr.Index(i).Set(newVal)
			}
		}
		v.Set(varr)
	}
	return
}

// setArrayWithIndexes handles decoding of array types with explicit indexes
func (d *decoder) setArrayWithIndexes(v reflect.Value, rd *recursiveData, namespace []byte) (set bool) {
	var varr reflect.Value
	var kv key

	overCapacity := rd.sliceLen >= v.Len()
	if overCapacity {
		// more values than array capacity, ignore values over capacity as it's possible some would just want
		// to grab the first x number of elements; in the future strict mode logic should return an error
		fmt.Println("warning number of post form array values is larger than array capacity, ignoring overflow values")
	}
	varr = reflect.Indirect(reflect.New(reflect.ArrayOf(v.Len(), v.Type().Elem())))
	reflect.Copy(varr, v)

	for i := 0; i < len(rd.keys); i++ {
		kv = rd.keys[i]
		if kv.ivalue >= v.Len() {
			continue
		}
		newVal := reflect.New(varr.Type().Elem()).Elem()

		if kv.ivalue == -1 {
			d.setError(namespace, fmt.Errorf("invalid array index '%s'", kv.value))
			continue
		}

		if d.setFieldByType(newVal, append(namespace, kv.searchValue...), 0) {
			set = true
			varr.Index(kv.ivalue).Set(newVal)
		}
	}

	if set {
		v.Set(varr)
	}
	return
}

// setMap handles decoding of map types
func (d *decoder) setMap(v reflect.Value, rd *recursiveData, namespace []byte) (set bool) {
	var existing bool
	var kv key
	var mp reflect.Value
	var mk reflect.Value

	typ := v.Type()

	if v.IsNil() {
		mp = reflect.MakeMap(typ)
	} else {
		existing = true
		mp = v
	}

	for i := 0; i < len(rd.keys); i++ {
		newVal := reflect.New(typ.Elem()).Elem()
		mk = reflect.New(typ.Key()).Elem()
		kv = rd.keys[i]

		if err := d.getMapKey(kv.value, mk, namespace); err != nil {
			d.setError(namespace, err)
			continue
		}

		if d.setFieldByType(newVal, append(namespace, kv.searchValue...), 0) {
			set = true
			mp.SetMapIndex(mk, newVal)
		}
	}

	if set && !existing {
		v.Set(mp)
	}
	return
}

func (d *decoder) setFieldByType(current reflect.Value, namespace []byte, idx int) (set bool) {

	var err error
	v, kind := ExtractType(current)

	// Convert namespace to string once to avoid repeated allocations
	ns := string(namespace)
	arr, ok := d.values[ns]

	if d.d.customTypeFuncs != nil {

		if ok {
			if cf, ok := d.d.customTypeFuncs[v.Type()]; ok {
				val, err := cf(arr[idx:])
				if err != nil {
					d.setError(namespace, err)
					return
				}

				v.Set(reflect.ValueOf(val))
				set = true
				return
			}
		}
	}
	switch kind {
	case reflect.Interface:
		if !ok || idx == len(arr) {
			return
		}
		v.Set(reflect.ValueOf(arr[idx]))
		set = true

	case reflect.Ptr:
		newVal := reflect.New(v.Type().Elem())
		if set = d.setFieldByType(newVal.Elem(), namespace, idx); set {
			v.Set(newVal)
		}

	case reflect.String:
		if !ok || idx == len(arr) {
			return
		}
		v.SetString(arr[idx])
		set = true

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64, reflect.Bool:
		if !ok {
			return
		}
		if set, err = d.setPrimitiveValue(v, arr, idx, kind, namespace, ns); err != nil {
			d.setError(namespace, err)
			return
		}

	case reflect.Slice:
		d.parseMapData()
		if ok {
			set = d.setSlice(v, arr, namespace, ns)
		}
		// maybe it's an numbered array i.e. Phone[0].Number
		if rd := d.findAlias(ns); rd != nil {
			if d.setSliceWithIndexes(v, rd, namespace) {
				set = true
			}
		}

	case reflect.Array:
		d.parseMapData()
		if ok {
			set = d.setArray(v, arr, namespace)
		}
		// maybe it's an numbered array i.e. Phone[0].Number
		if rd := d.findAlias(ns); rd != nil {
			if d.setArrayWithIndexes(v, rd, namespace) {
				set = true
			}
		}

	case reflect.Map:
		d.parseMapData()
		// no natural map support so skip directly to dm lookup
		if rd := d.findAlias(ns); rd != nil {
			set = d.setMap(v, rd, namespace)
		}

	case reflect.Struct:
		typ := v.Type()

		// if we get here then no custom time function declared so use RFC3339 by default
		if typ == timeType {

			if !ok || len(arr[idx]) == 0 {
				return
			}

			t, err := time.Parse(time.RFC3339, arr[idx])
			if err != nil {
				d.setError(namespace, err)
			}

			v.Set(reflect.ValueOf(t))
			set = true
			return
		}

		d.parseMapData()

		// we must be recursing infinitly...but that's ok we caught it on the very first overun.
		if len(namespace) > d.maxKeyLen {
			return
		}

		set = d.traverseStruct(v, typ, namespace)
	}
	return
}

func (d *decoder) getMapKey(key string, current reflect.Value, namespace []byte) (err error) {

	v, kind := ExtractType(current)

	// Convert namespace to string once to avoid repeated allocations
	ns := string(namespace)

	if d.d.customTypeFuncs != nil {
		if cf, ok := d.d.customTypeFuncs[v.Type()]; ok {

			val, er := cf([]string{key})
			if er != nil {
				err = er
				return
			}

			v.Set(reflect.ValueOf(val))
			return
		}
	}

	switch kind {
	case reflect.Interface:
		// If interface would have been set on the struct before decoding,
		// say to a struct value we would not get here but kind would be struct.
		v.Set(reflect.ValueOf(key))
		return
	case reflect.Ptr:
		newVal := reflect.New(v.Type().Elem())
		if err = d.getMapKey(key, newVal.Elem(), namespace); err == nil {
			v.Set(newVal)
		}

	case reflect.String:
		v.SetString(key)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64, reflect.Bool:
		err = d.setPrimitiveMapKey(v, key, kind, ns)

	default:
		err = fmt.Errorf("Unsupported Map Key '%s', Type '%v' Namespace '%s'", key, v.Type(), ns)
	}

	return
}
