package middleware

import (
	"context"
	"net/http"

	"github.com/redis/go-redis/v9"
)

var budgetLua = `
local budget_key = KEYS[1]
local budget = redis.call('GET', budget_key)

if not budget then
    return -1 -- No budget found (assume 0)
end

local b = tonumber(budget)
if b <= 0 then
    return -1 -- Budget exhausted
end

-- For V1, decrement by 1 unit per request as a placeholder for actual token cost calculation
local new_budget = b - 1
redis.call('SET', budget_key, tostring(new_budget))

return new_budget
`

func BudgetGuard(redisClient *redis.Client, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.Header.Get("X-Team-ID")
		if teamID == "" {
			http.Error(w, "X-Team-ID header missing", http.StatusUnauthorized)
			return
		}

		ctx := context.Background()
		key := "budget:team:" + teamID

		// Execute Lua script
		result, err := redisClient.Eval(ctx, budgetLua, []string{key}).Result()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		newBudget := result.(int64)
		if newBudget < 0 {
			http.Error(w, "Insufficient Budget", http.StatusPaymentRequired) // HTTP 402
			return
		}

		next.ServeHTTP(w, r)
	}
}
