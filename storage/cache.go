package storage

import (
	"time"
	"sync"
	"fmt"
	"io"
	"encoding/gob"
	"os"
	"runtime"
)

type Item struct {
	Object     interface{}
	Expiration int64
}

func (item Item) Expired() bool {
	if item.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expiration
}

const (
	// For use with functions that take an expiration time.
	NoExpiration time.Duration = -1
	// For use with functions that take an expiration time. Equivalent to
	// passing in the same expiration duration as was given to New() or
	// NewFrom() when the Cache was created (expire.g. 5 minutes.)
	DefaultExpiration time.Duration = 0
)

type Cache struct {
	defaultExpiration time.Duration
	items             map[string]Item
	mutex             sync.RWMutex
	onEvicted         func(string, interface{})
	janitor           *janitor
}

/*
添加或者替换
timeout: 0  -> DefaultExpiration
         -1 -> NoExpiration       永不过期
*/
func (c *Cache) Set(key string, value interface{}, timeout time.Duration) {
	var expire int64
	if timeout == DefaultExpiration {
		timeout = c.defaultExpiration
	}
	if timeout > 0 {
		expire = time.Now().Add(timeout).UnixNano()
	}
	c.mutex.Lock()
	c.items[key] = Item{
		Object:     value,
		Expiration: expire,
	}
	// TODO: Calls to mutex.Unlock are currently not deferred because defer
	// adds ~200 ns (as of go1.)
	c.mutex.Unlock()
}

// 默认过期时间
func (c *Cache) SetDefault(key string, value interface{}) {
	c.Set(key, value, DefaultExpiration)
}

func (c *Cache) set(key string, value interface{}, timeout time.Duration) {
	var expire int64
	if timeout == DefaultExpiration {
		timeout = c.defaultExpiration
	}
	if timeout > 0 {
		expire = time.Now().Add(timeout).UnixNano()
	}
	c.items[key] = Item{
		Object:     value,
		Expiration: expire,
	}
}

func (c *Cache) Add(key string, value interface{}, timeout time.Duration) error {
	c.mutex.Lock()
	_, found := c.get(key)
	if found {
		c.mutex.Unlock()
		return fmt.Errorf("item %s already exists", key)
	}

	c.set(key, value, timeout)
	c.mutex.Unlock()

	return nil
}

func (c *Cache) Replace(key string, value interface{}, timeout time.Duration) error {
	c.mutex.Lock()
	_, found := c.get(key)
	if !found {
		c.mutex.Unlock()
		return fmt.Errorf("item %s doesn't exist", key)
	}

	c.set(key, value, timeout)
	c.mutex.Unlock()

	return nil
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	item, found := c.items[key]
	if !found {
		c.mutex.RUnlock()
		return nil, false
	}

	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.mutex.RUnlock()
			return nil, false
		}
	}
	c.mutex.RUnlock()

	return item.Object, true
}

// GetWithExpiration returns an item and its expiration time from the Cache.
// It returns the item or nil, the expiration time if one is set (if the item
// never expires a zero value for time.Time is returned), and a bool indicating
// whether the key was found.
func (c *Cache) GetWithExpiration(key string) (interface{}, time.Time, bool) {
	c.mutex.RLock()
	item, found := c.items[key]
	if !found {
		c.mutex.RUnlock()
		return nil, time.Time{}, false
	}

	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.mutex.RUnlock()
			return nil, time.Time{}, false
		}

		// Return the item and the expiration time
		c.mutex.RUnlock()
		return item.Object, time.Unix(0, item.Expiration), true
	}

	// If expiration <= 0 (i.expire. no expiration time set) then return the item
	// and a zeroed time.Time
	c.mutex.RUnlock()
	return item.Object, time.Time{}, true
}

func (c *Cache) get(key string) (interface{}, bool) {
	item, found := c.items[key]
	if !found {
		return nil, false
	}
	// "Inlining" of Expired
	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			return nil, false
		}
	}
	return item.Object, true
}

// Increment an item of type int, int8, int16, int32, int64, uintptr, uint,
// uint8, uint32, or uint64, float32 or float64 by n. Returns an error if the
// item's value is not an integer, if it was not found, or if it is not
// possible to increment it by n. To retrieve the incremented value, use one
// of the specialized methods, expire.g. IncrementInt64.
func (c *Cache) Increment(key string, n int64) error {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return fmt.Errorf("Item %s not found", key)
	}
	switch v.Object.(type) {
	case int:
		v.Object = v.Object.(int) + int(n)
	case int8:
		v.Object = v.Object.(int8) + int8(n)
	case int16:
		v.Object = v.Object.(int16) + int16(n)
	case int32:
		v.Object = v.Object.(int32) + int32(n)
	case int64:
		v.Object = v.Object.(int64) + n
	case uint:
		v.Object = v.Object.(uint) + uint(n)
	case uintptr:
		v.Object = v.Object.(uintptr) + uintptr(n)
	case uint8:
		v.Object = v.Object.(uint8) + uint8(n)
	case uint16:
		v.Object = v.Object.(uint16) + uint16(n)
	case uint32:
		v.Object = v.Object.(uint32) + uint32(n)
	case uint64:
		v.Object = v.Object.(uint64) + uint64(n)
	case float32:
		v.Object = v.Object.(float32) + float32(n)
	case float64:
		v.Object = v.Object.(float64) + float64(n)
	default:
		c.mutex.Unlock()
		return fmt.Errorf("The value for %s is not an integer", key)
	}
	c.items[key] = v
	c.mutex.Unlock()
	return nil
}

// Increment an item of type float32 or float64 by n. Returns an error if the
// item's value is not floating point, if it was not found, or if it is not
// possible to increment it by n. Pass a negative number to decrement the
// value. To retrieve the incremented value, use one of the specialized methods,
// expire.g. IncrementFloat64.
func (c *Cache) IncrementFloat(key string, n float64) error {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return fmt.Errorf("Item %s not found", key)
	}
	switch v.Object.(type) {
	case float32:
		v.Object = v.Object.(float32) + float32(n)
	case float64:
		v.Object = v.Object.(float64) + n
	default:
		c.mutex.Unlock()
		return fmt.Errorf("The value for %s does not have type float32 or float64", key)
	}
	c.items[key] = v
	c.mutex.Unlock()
	return nil
}

// Increment an item of type int by n. Returns an error if the item's value is
// not an int, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *Cache) IncrementInt(key string, n int) (int, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(int)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an int", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type int8 by n. Returns an error if the item's value is
// not an int8, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *Cache) IncrementInt8(key string, n int8) (int8, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(int8)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an int8", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type int16 by n. Returns an error if the item's value is
// not an int16, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *Cache) IncrementInt16(key string, n int16) (int16, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(int16)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an int16", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type int32 by n. Returns an error if the item's value is
// not an int32, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *Cache) IncrementInt32(key string, n int32) (int32, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(int32)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an int32", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type int64 by n. Returns an error if the item's value is
// not an int64, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *Cache) IncrementInt64(key string, n int64) (int64, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(int64)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an int64", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type uint by n. Returns an error if the item's value is
// not an uint, or if it was not found. If there is no error, the incremented
// value is returned.
func (c *Cache) IncrementUint(key string, n uint) (uint, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(uint)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uint", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type uintptr by n. Returns an error if the item's value
// is not an uintptr, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *Cache) IncrementUintptr(key string, n uintptr) (uintptr, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(uintptr)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uintptr", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type uint8 by n. Returns an error if the item's value
// is not an uint8, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *Cache) IncrementUint8(key string, n uint8) (uint8, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(uint8)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uint8", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type uint16 by n. Returns an error if the item's value
// is not an uint16, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *Cache) IncrementUint16(key string, n uint16) (uint16, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(uint16)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uint16", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type uint32 by n. Returns an error if the item's value
// is not an uint32, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *Cache) IncrementUint32(key string, n uint32) (uint32, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(uint32)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uint32", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type uint64 by n. Returns an error if the item's value
// is not an uint64, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *Cache) IncrementUint64(key string, n uint64) (uint64, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(uint64)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an uint64", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type float32 by n. Returns an error if the item's value
// is not an float32, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *Cache) IncrementFloat32(key string, n float32) (float32, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(float32)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an float32", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Increment an item of type float64 by n. Returns an error if the item's value
// is not an float64, or if it was not found. If there is no error, the
// incremented value is returned.
func (c *Cache) IncrementFloat64(key string, n float64) (float64, error) {
	c.mutex.Lock()
	v, found := c.items[key]
	if !found || v.Expired() {
		c.mutex.Unlock()
		return 0, fmt.Errorf("Item %s not found", key)
	}
	rv, ok := v.Object.(float64)
	if !ok {
		c.mutex.Unlock()
		return 0, fmt.Errorf("The value for %s is not an float64", key)
	}
	nv := rv + n
	v.Object = nv
	c.items[key] = v
	c.mutex.Unlock()
	return nv, nil
}

// Delete an item from the Cache. Does nothing if the key is not in the Cache.
func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	v, evicted := c.delete(key)
	c.mutex.Unlock()
	if evicted {
		c.onEvicted(key, v)
	}
}

func (c *Cache) delete(key string) (interface{}, bool) {
	if c.onEvicted != nil {
		if v, found := c.items[key]; found {
			delete(c.items, key)
			return v.Object, true
		}
	}
	delete(c.items, key)
	return nil, false
}

type keyAndValue struct {
	key   string
	value interface{}
}

// Delete all expired items from the Cache.
func (c *Cache) DeleteExpired() {
	var evictedItems []keyAndValue
	now := time.Now().UnixNano()
	c.mutex.Lock()
	for key, v := range c.items {
		// "Inlining" of expired
		if v.Expiration > 0 && now > v.Expiration {
			ov, evicted := c.delete(key)
			if evicted {
				evictedItems = append(evictedItems, keyAndValue{key, ov})
			}
		}
	}
	c.mutex.Unlock()
	for _, v := range evictedItems {
		c.onEvicted(v.key, v.value)
	}
}

// Sets an (optional) function that is called with the key and value when an
// item is evicted from the Cache. (Including when it is deleted manually, but
// not when it is overwritten.) Set to nil to disable.
func (c *Cache) OnEvicted(f func(string, interface{})) {
	c.mutex.Lock()
	c.onEvicted = f
	c.mutex.Unlock()
}

// Write the Cache's items (using Gob) to an io.Writer.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *Cache) Save(w io.Writer) (err error) {
	enc := gob.NewEncoder(w)
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("Error registering item types with Gob library")
		}
	}()
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	for _, v := range c.items {
		gob.Register(v.Object)
	}
	err = enc.Encode(&c.items)
	return
}

// Save the Cache's items to the given filename, creating the file if it
// doesn't exist, and overwriting it if it does.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *Cache) SaveFile(fname string) error {
	fp, err := os.Create(fname)
	if err != nil {
		return err
	}
	err = c.Save(fp)
	if err != nil {
		fp.Close()
		return err
	}
	return fp.Close()
}

// Add (Gob-serialized) Cache items from an io.Reader, excluding any items with
// keys that already exist (and haven't expired) in the current Cache.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *Cache) Load(r io.Reader) error {
	dec := gob.NewDecoder(r)
	items := map[string]Item{}
	err := dec.Decode(&items)
	if err == nil {
		c.mutex.Lock()
		defer c.mutex.Unlock()
		for key, v := range items {
			ov, found := c.items[key]
			if !found || ov.Expired() {
				c.items[key] = v
			}
		}
	}
	return err
}

// Load and add Cache items from the given filename, excluding any items with
// keys that already exist in the current Cache.
//
// NOTE: This method is deprecated in favor of c.Items() and NewFrom() (see the
// documentation for NewFrom().)
func (c *Cache) LoadFile(fname string) error {
	fp, err := os.Open(fname)
	if err != nil {
		return err
	}
	err = c.Load(fp)
	if err != nil {
		fp.Close()
		return err
	}
	return fp.Close()
}

// Copies all unexpired items in the Cache into a new map and returns it.
func (c *Cache) Items() map[string]Item {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	m := make(map[string]Item, len(c.items))
	now := time.Now().UnixNano()
	for key, v := range c.items {
		// "Inlining" of Expired
		if v.Expiration > 0 {
			if now > v.Expiration {
				continue
			}
		}
		m[key] = v
	}
	return m
}

// Returns the number of items in the Cache. This may include items that have
// expired, but have not yet been cleaned up.
func (c *Cache) ItemCount() int {
	c.mutex.RLock()
	n := len(c.items)
	c.mutex.RUnlock()
	return n
}

// Delete all items from the Cache.
func (c *Cache) Flush() {
	c.mutex.Lock()
	c.items = map[string]Item{}
	c.mutex.Unlock()
}

type janitor struct {
	Interval time.Duration
	stop     chan bool
}

func (j *janitor) Run(c *Cache) {
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopJanitor(c *Cache) {
	c.janitor.stop <- true
}

func runJanitor(c *Cache, ci time.Duration) {
	j := &janitor{
		Interval: ci,
		stop:     make(chan bool),
	}
	c.janitor = j
	go j.Run(c)
}

func newCache(de time.Duration, m map[string]Item) *Cache {
	if de == 0 {
		de = -1
	}
	c := &Cache{
		defaultExpiration: de,
		items:             m,
	}
	return c
}

func newCacheWithJanitor(de time.Duration, ci time.Duration, m map[string]Item) *Cache {
	c := newCache(de, m)
	// This trick ensures that the janitor goroutine (which--granted it
	// was enabled--is running DeleteExpired on c forever) does not keep
	// the returned C object from being garbage collected. When it is
	// garbage collected, the finalizer stops the janitor goroutine, after
	// which c can be collected.
	if ci > 0 {
		runJanitor(c, ci)
		runtime.SetFinalizer(c, stopJanitor)
	}
	return c
}

// Return a new Cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration),
// the items in the Cache never expire (by default), and must be deleted
// manually. If the cleanup interval is less than one, expired items are not
// deleted from the Cache before calling c.DeleteExpired().
func New(defaultExpiration, cleanupInterval time.Duration) *Cache {
	items := make(map[string]Item)
	return newCacheWithJanitor(defaultExpiration, cleanupInterval, items)
}

// Return a new Cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration),
// the items in the Cache never expire (by default), and must be deleted
// manually. If the cleanup interval is less than one, expired items are not
// deleted from the Cache before calling c.DeleteExpired().
//
// NewFrom() also accepts an items map which will serve as the underlying map
// for the Cache. This is useful for starting from a deserialized Cache
// (serialized using expire.g. gob.Encode() on c.Items()), or passing in expire.g.
// make(map[string]Item, 500) to improve startup performance when the Cache
// is expected to reach a certain minimum size.
//
// Only the Cache's methods synchronize access to this map, so it is not
// recommended to keep any references to the map around after creating a Cache.
// If need be, the map can be accessed at a later point using c.Items() (subject
// to the same caveat.)
//
// Note regarding serialization: When using expire.g. gob, make sure to
// gob.Register() the individual types stored in the Cache before encoding a
// map retrieved with c.Items(), and to register those same types before
// decoding a blob containing an items map.
func NewFrom(defaultExpiration, cleanupInterval time.Duration, items map[string]Item) *Cache {
	return newCacheWithJanitor(defaultExpiration, cleanupInterval, items)
}
