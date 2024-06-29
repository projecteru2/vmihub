package utils

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/projecteru2/vmihub/internal/utils/idgen"
	"golang.org/x/crypto/bcrypt"
)

func EncryptPassword(passwd string) (string, error) {
	// 加密密码
	bs, err := bcrypt.GenerateFromPassword([]byte(passwd), bcrypt.DefaultCost)
	return string(bs), err

}

func EnsureDir(d string) error {
	err := os.MkdirAll(d, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func GenerateUniqueID(input string) string {
	sum := sha256.Sum256([]byte(input))
	uniqueID := hex.EncodeToString(sum[:])
	return uniqueID
}

func GetBooleanQuery(c *gin.Context, key string, dValue bool) bool {
	if value, exists := c.GetQuery(key); exists {
		return value == "true"
	}
	return dValue
}

func WithTimeout(ctx context.Context, timeout time.Duration, f func(ctx2 context.Context)) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	f(ctx)
}

func GetUniqueStr() (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", id), nil
}

func RandomString(length int) string {
	b := make([]byte, length+2)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)[2 : length+2]
}

// RadomStringByPhone 根据用户电话号码以及当前时间生成随机字符串,8位
func RadomStringByPhone(phone string) string {
	source := fmt.Sprintf("%s%d", phone, time.Now().UnixNano())
	hash := sha256.New()
	hash.Write([]byte(source))
	hashedPhone := hash.Sum(nil)
	hexString := hex.EncodeToString(hashedPhone)
	return hexString[:8]
}

func GetUniqueSID() string {
	// Generate a snowflake ID.
	return idgen.NextSID()
}

func GetExpiredTimeByPeriod(period string) (*time.Time, error) {
	timeDuration := ""
	switch {
	case strings.HasSuffix(period, "h"):
		timeDuration = strings.TrimRight(period, "h")
	case strings.HasSuffix(period, "m"):
		timeDuration = strings.TrimRight(period, "m")
	case strings.HasSuffix(period, "y"):
		timeDuration = strings.TrimRight(period, "y")
	default:
		return nil, errors.New("period error")
	}
	timeDurationVal, err := strconv.Atoi(timeDuration)
	if err != nil {
		return nil, errors.New("period error")
	}
	if timeDurationVal <= 0 {
		return nil, errors.New("period error")
	}
	nowTime := time.Now()
	switch {
	case strings.HasSuffix(period, "m"):
		expiredTime := nowTime.AddDate(0, timeDurationVal, 0)
		return &expiredTime, nil
	case strings.HasSuffix(period, "y"):
		expiredTime := nowTime.AddDate(timeDurationVal, 0, 0)
		return &expiredTime, nil
	}
	return nil, errors.New("period error")
}

func GetMaxTime() time.Time {
	maxTime := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	return maxTime
}

func GetInternalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("get Interfaces error：%v", err)
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			fmt.Println("get Addrs error：", err)
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok && !ipNet.IP.IsLoopback() {
				if ipNet.IP.To4() != nil {
					return ipNet.IP.String(), nil
				}
			}
		}
	}

	return "", fmt.Errorf("error")
}

func Invoke(fn func() error) error {
	return fn()
}

func Contains(sli []string, str string) bool {
	for _, value := range sli {
		if value == str {
			return true
		}
	}
	return false
}

// RoundMoney 四舍五入，小数点2位
func RoundMoney(v float64) float64 {
	return math.Round(v*math.Pow(10, 2)) / math.Pow(10, 2)
}

func UUIDStr() (string, error) {
	uuidRaw, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(uuidRaw[:]), nil
}

// GenRandomBigString 生成指定长度大写字母组成的随机字符串
func GenRandomBigString(length int) (string, error) {
	const uppercaseLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	for i := 0; i < length; i++ {
		randomBytes[i] = uppercaseLetters[int(randomBytes[i])%len(uppercaseLetters)]
	}
	return string(randomBytes), nil
}

// UUIDStrNew 六位随机大写字母_毫秒字符串
func UUIDStrNew() (string, error) {
	randomStr, err := GenRandomBigString(6)
	milliSec := time.Now().UnixNano() / int64(time.Millisecond)
	return fmt.Sprintf("%s_%d", randomStr, milliSec), err
}
