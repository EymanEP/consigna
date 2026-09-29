// Package session manages who is in the tray: the join code, the devices
// that used it, their credentials and whether they are online.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"math/big"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/EymanEP/consigna/internal/names"
)

// Errors returned by the manager.
var (
	ErrInvalidCode = errors.New("session: invalid code")
	ErrRateLimited = errors.New("session: too many attempts")
	ErrFull        = errors.New("session: too many devices")
	ErrNotFound    = errors.New("session: device not found")
	ErrInvalidName = errors.New("session: invalid name")
	ErrNameTaken   = errors.New("session: name already in use")
)

// CodeLength is the number of characters in a join code.
const CodeLength = 6

// codeAlphabet leaves out characters that are easy to confuse (0/O, 1/I/L, U/V).
const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTWXYZ"

// MaxNameRunes bounds device names.
const MaxNameRunes = 32

// maxHostDevices bounds the devices admitted without a code (the host's own
// browsers). Beyond it, the least recently seen offline one is replaced, so
// stray requests can never crowd out the slots real devices join into.
const maxHostDevices = 8

// Device is a browser that joined the session.
type Device struct {
	ID        string
	Name      string
	IP        string
	UserAgent string
	IsHost    bool
	Online    bool
	JoinedAt  time.Time
	LastSeen  time.Time
}

type device struct {
	Device
	token [sha256.Size]byte
	conns int
}

// Options configure a Manager.
type Options struct {
	// Now returns the current time. Defaults to time.Now.
	Now func() time.Time
	// OnChange is called, outside any lock, when the set of devices, their
	// names or their online state changes.
	OnChange func()
	// MaxDevices caps how many devices may join. Defaults to 256.
	MaxDevices int
	// FailuresPerIP and GlobalFailures bound wrong-code attempts within
	// FailureWindow. Defaults: 10 per IP, 200 overall, 10 minutes.
	FailuresPerIP  int
	GlobalFailures int
	FailureWindow  time.Duration
}

// Manager is safe for concurrent use.
type Manager struct {
	opts Options

	mu      sync.Mutex
	code    string
	byID    map[string]*device
	byToken map[[sha256.Size]byte]*device
	limiter *limiter
}

// New creates a manager with a fresh join code.
func New(opts Options) *Manager {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.OnChange == nil {
		opts.OnChange = func() {}
	}
	if opts.MaxDevices <= 0 {
		opts.MaxDevices = 256
	}
	if opts.FailuresPerIP <= 0 {
		opts.FailuresPerIP = 10
	}
	if opts.GlobalFailures <= 0 {
		opts.GlobalFailures = 200
	}
	if opts.FailureWindow <= 0 {
		opts.FailureWindow = 10 * time.Minute
	}
	return &Manager{
		opts:    opts,
		code:    newCode(),
		byID:    map[string]*device{},
		byToken: map[[sha256.Size]byte]*device{},
		limiter: newLimiter(opts.FailuresPerIP, opts.GlobalFailures, opts.FailureWindow),
	}
}

// Code returns the current join code, upper case.
func (m *Manager) Code() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.code
}

// Join admits a new device if code matches the current join code. It returns
// the device's secret token, which the caller stores in a cookie.
func (m *Manager) Join(code, ip, userAgent string) (string, Device, error) {
	now := m.opts.Now()
	m.mu.Lock()
	if !m.limiter.allowed(ip, now) {
		m.mu.Unlock()
		return "", Device{}, ErrRateLimited
	}
	given := strings.ToUpper(strings.TrimSpace(code))
	if subtle.ConstantTimeCompare([]byte(given), []byte(m.code)) != 1 {
		m.limiter.fail(ip, now)
		m.mu.Unlock()
		return "", Device{}, ErrInvalidCode
	}
	token, d, err := m.addLocked(ip, userAgent, false, now)
	m.mu.Unlock()
	if err == nil {
		m.opts.OnChange()
	}
	return token, d, err
}

// JoinTrusted admits a device without a code. It is used for the host's own
// browser, which connects over loopback.
func (m *Manager) JoinTrusted(ip, userAgent string, isHost bool) (string, Device, error) {
	now := m.opts.Now()
	m.mu.Lock()
	if isHost {
		m.evictStaleHostLocked()
	}
	token, d, err := m.addLocked(ip, userAgent, isHost, now)
	m.mu.Unlock()
	if err == nil {
		m.opts.OnChange()
	}
	return token, d, err
}

// evictStaleHostLocked makes room for a new host device when the host limit
// is reached, by dropping the offline host device seen longest ago.
func (m *Manager) evictStaleHostLocked() {
	var hosts int
	var oldest *device
	for _, d := range m.byID {
		if !d.IsHost {
			continue
		}
		hosts++
		if d.conns == 0 && (oldest == nil || d.LastSeen.Before(oldest.LastSeen)) {
			oldest = d
		}
	}
	if hosts >= maxHostDevices && oldest != nil {
		delete(m.byID, oldest.ID)
		delete(m.byToken, oldest.token)
	}
}

func (m *Manager) addLocked(ip, userAgent string, isHost bool, now time.Time) (string, Device, error) {
	if len(m.byID) >= m.opts.MaxDevices {
		return "", Device{}, ErrFull
	}
	token := randomToken()
	d := &device{
		Device: Device{
			ID:        randomID(),
			Name:      names.Generate(m.nameTakenLocked("")),
			IP:        ip,
			UserAgent: truncate(userAgent, 256),
			IsHost:    isHost,
			JoinedAt:  now,
			LastSeen:  now,
		},
		token: sha256.Sum256([]byte(token)),
	}
	m.byID[d.ID] = d
	m.byToken[d.token] = d
	return token, d.Device, nil
}

// Authenticate resolves a token to its device and records the activity.
func (m *Manager) Authenticate(token, ip, userAgent string) (Device, bool) {
	if token == "" {
		return Device{}, false
	}
	key := sha256.Sum256([]byte(token))
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.byToken[key]
	if !ok {
		return Device{}, false
	}
	d.LastSeen = m.opts.Now()
	if ip != "" {
		d.IP = ip
	}
	if userAgent != "" {
		d.UserAgent = truncate(userAgent, 256)
	}
	return d.snapshot(), true
}

// Get returns a device by ID.
func (m *Manager) Get(id string) (Device, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.byID[id]
	if !ok {
		return Device{}, false
	}
	return d.snapshot(), true
}

// Devices returns all devices in join order.
func (m *Manager) Devices() []Device {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Device, 0, len(m.byID))
	for _, d := range m.byID {
		out = append(out, d.snapshot())
	}
	slices.SortFunc(out, func(a, b Device) int {
		if c := a.JoinedAt.Compare(b.JoinedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// Rename changes a device's display name.
func (m *Manager) Rename(id, name string) (Device, error) {
	name, err := CleanName(name)
	if err != nil {
		return Device{}, err
	}
	m.mu.Lock()
	d, ok := m.byID[id]
	if !ok {
		m.mu.Unlock()
		return Device{}, ErrNotFound
	}
	if m.nameTakenLocked(id)(name) {
		m.mu.Unlock()
		return Device{}, ErrNameTaken
	}
	changed := d.Name != name
	d.Name = name
	snap := d.snapshot()
	m.mu.Unlock()
	if changed {
		m.opts.OnChange()
	}
	return snap, nil
}

// Connect marks a device as online until the returned release function is
// called. A device with several open tabs stays online until all close.
func (m *Manager) Connect(id string) (release func()) {
	m.mu.Lock()
	d, ok := m.byID[id]
	if !ok {
		m.mu.Unlock()
		return func() {}
	}
	d.conns++
	cameOnline := d.conns == 1
	m.mu.Unlock()
	if cameOnline {
		m.opts.OnChange()
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			cur, ok := m.byID[id]
			wentOffline := false
			if ok && cur == d && d.conns > 0 {
				d.conns--
				wentOffline = d.conns == 0
			}
			m.mu.Unlock()
			if wentOffline {
				m.opts.OnChange()
			}
		})
	}
}

// Remove signs a device out. Its token stops working immediately.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	d, ok := m.byID[id]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	delete(m.byID, id)
	delete(m.byToken, d.token)
	m.mu.Unlock()
	m.opts.OnChange()
	return nil
}

// RotateCode replaces the join code. Devices already in stay in.
func (m *Manager) RotateCode() string {
	m.mu.Lock()
	m.code = newCode()
	code := m.code
	m.mu.Unlock()
	m.opts.OnChange()
	return code
}

// End signs every device out and replaces the join code.
func (m *Manager) End() string {
	m.mu.Lock()
	m.byID = map[string]*device{}
	m.byToken = map[[sha256.Size]byte]*device{}
	m.code = newCode()
	code := m.code
	m.mu.Unlock()
	m.opts.OnChange()
	return code
}

// CleanName normalises a user-chosen device name, rejecting empty, overlong
// or invisible names.
func CleanName(name string) (string, error) {
	var b strings.Builder
	space := false
	for _, r := range strings.TrimSpace(name) {
		switch {
		case unicode.IsSpace(r):
			space = true
			continue
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r):
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	out := b.String()
	if out == "" || utf8.RuneCountInString(out) > MaxNameRunes {
		return "", ErrInvalidName
	}
	return out, nil
}

func (m *Manager) nameTakenLocked(exceptID string) func(string) bool {
	return func(name string) bool {
		for _, d := range m.byID {
			if d.ID != exceptID && strings.EqualFold(d.Name, name) {
				return true
			}
		}
		return false
	}
}

func (d *device) snapshot() Device {
	s := d.Device
	s.Online = d.conns > 0
	return s
}

func newCode() string {
	b := make([]byte, CodeLength)
	max := big.NewInt(int64(len(codeAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err) // crypto/rand never fails on supported platforms
		}
		b[i] = codeAlphabet[n.Int64()]
	}
	return string(b)
}

func randomToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func randomID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	s = s[:max]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
