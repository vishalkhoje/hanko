package rate_limiter

import (
	"context"
	"github.com/gofrs/uuid"
	"github.com/gomodule/redigo/redis"
	"github.com/labstack/echo/v4"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
	"github.com/sethvargo/go-redisstore"
	"github.com/teamhanko/hanko/backend/config"
	"github.com/teamhanko/hanko/backend/dto"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"
)

func NewRateLimiter(cfg config.RateLimiter) limiter.Store {
	if cfg.Backend == config.RATE_LIMITER_BACKEND_REDIS {
		//ctx := context.Background()
		store, err := redisstore.New(&redisstore.Config{
			Tokens:   *cfg.Tokens,
			Interval: *cfg.Interval,
			Dial: func() (redis.Conn, error) {
				return redis.Dial("tcp", cfg.Redis.Address,
					redis.DialPassword(cfg.Redis.Password))
			},
		})
		if err != nil {
			log.Fatal(err)
		}
		//defer store.Close(ctx)
		return store
	}
	// else return in_memory
	store, err := memorystore.New(&memorystore.Config{
		// Number of tokens allowed per interval.
		Tokens: *cfg.Tokens,

		// Interval until tokens reset.
		Interval: *cfg.Interval,
	})
	if err != nil {
		log.Fatal(err)
	}
	return store
}

func Limit(store limiter.Store, userId uuid.UUID, c echo.Context) error {
	key := userId.String() + "/" + c.RealIP()
	// Take from the store.
	limit, remaining, reset, ok, err := store.Take(context.Background(), key)

	if err != nil {
		return err
	}

	//resetTime := time.Unix(0, int64(reset)).UTC().Format(time.RFC1123)
	resetTime := int(math.Floor(time.Unix(0, int64(reset)).UTC().Sub(time.Now().UTC()).Seconds()))
	log.Println(resetTime)

	// Set headers (we do this regardless of whether the request is permitted).
	c.Response().Header().Set(config.HeaderRateLimitLimit, strconv.FormatUint(limit, 10))
	c.Response().Header().Set(config.HeaderRateLimitRemaining, strconv.FormatUint(remaining, 10))
	c.Response().Header().Set(config.HeaderRateLimitReset, strconv.Itoa(resetTime))

	// Fail if there were no tokens remaining.
	if !ok {
		c.Response().Header().Set(config.HeaderRetryAfter, strconv.Itoa(resetTime))
		return dto.NewHTTPError(http.StatusTooManyRequests)
	}
	return nil
}
