package middleware

import (
	"context"
	"log"
	"net/http"

	"github.com/finops-gateway/internal/ledger"
	"github.com/finops-gateway/internal/schema"
	"github.com/redis/go-redis/v9"
)

// Pricing definitions (Cost per 1 Million Tokens in USD)
type ModelPricing struct {
	InputCost  float64
	OutputCost float64
}

// Map of known models to their pricing per 1M tokens
var PricingTable = map[string]ModelPricing{
	"gpt-4o":                     {InputCost: 5.00, OutputCost: 15.00},
	"claude-3-5-sonnet":          {InputCost: 3.00, OutputCost: 15.00},
	"poolside/laguna-s-2.1:free": {InputCost: 1.00, OutputCost: 1.00}, // Faked paid cost for demo purposes
}

// budgetCheckLua simply reads the budget without decrementing
var budgetCheckLua = `
local budget = redis.call('GET', KEYS[1])
if not budget then return -1 end
return tonumber(budget)
`

// budgetDeductLua deducts the exact cost
var budgetDeductLua = `
local budget_key = KEYS[1]
local cost = tonumber(ARGV[1])
local budget = redis.call('GET', budget_key)
if not budget then return -1 end
local new_budget = tonumber(budget) - cost
redis.call('SET', budget_key, tostring(new_budget))
return new_budget
`

const TokenUsageKey string = "token_usage_ptr"

func BudgetGuard(redisClient *redis.Client, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.Header.Get("X-Team-ID")
		if teamID == "" {
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.Background()
		key := "budget:team:" + teamID

		// 1. PRE-FLIGHT: Check if they have budget
		result, err := redisClient.Eval(ctx, budgetCheckLua, []string{key}).Result()
		if err != nil && err != redis.Nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if result != nil {
			budget := result.(int64)
			if budget <= 0 {
				log.Printf("[BudgetGuard] Request Rejected: Insufficient Funds for %s (Balance: %d micro-dollars)", teamID, budget)
				if rec, ok := r.Context().Value(LogRecordKey).(*ledger.AuditRecord); ok {
					rec.BlockedReason = "Insufficient Budget"
				}
				http.Error(w, "Insufficient Budget", http.StatusPaymentRequired) // HTTP 402
				return
			}
		}

		// 2. Inject Context Pointer for downstream Adapter to populate
		usage := &schema.TokenUsage{}
		reqCtx := context.WithValue(r.Context(), TokenUsageKey, usage)
		r = r.WithContext(reqCtx)

		// 3. Execute Downstream Middleware
		next.ServeHTTP(w, r)

		// 4. POST-FLIGHT: Read Usage Pointer, Calculate Cost and Deduct
		if usage.Model != "" && usage.TotalTokens > 0 {
			cost := calculateMicroDollarCost(usage.Model, usage.PromptTokens, usage.CompletionTokens)
			if cost > 0 {
				newBudget, evalErr := redisClient.Eval(ctx, budgetDeductLua, []string{key}, cost).Result()
				if evalErr == nil {
					log.Printf("[BudgetGuard] Team %s spent %d micro-dollars on %s (%d in, %d out). Remaining: %d micro-dollars", 
						teamID, cost, usage.Model, usage.PromptTokens, usage.CompletionTokens, newBudget.(int64))
				}
				
				// Inject into AuditRecord if it exists in Context
				if rec, ok := r.Context().Value(LogRecordKey).(*ledger.AuditRecord); ok {
					rec.Model = usage.Model
					rec.MicroDollarCost = cost
					rec.PromptTokens = usage.PromptTokens
					rec.CompletionTokens = usage.CompletionTokens
					rec.TotalTokens = usage.TotalTokens
				}
			}
		}
	}
}

// Calculate the cost in micro-dollars (1 dollar = 1,000,000 micro-dollars)
func calculateMicroDollarCost(model string, promptTokens, completionTokens int) int64 {
	pricing, ok := PricingTable[model]
	if !ok {
		// Fallback pricing if model unknown (e.g., $1.00 per 1M tokens)
		pricing = ModelPricing{InputCost: 1.0, OutputCost: 1.0}
	}

	costPerInputToken := pricing.InputCost / 1000000.0
	costPerOutputToken := pricing.OutputCost / 1000000.0

	totalCostDollars := (float64(promptTokens) * costPerInputToken) + (float64(completionTokens) * costPerOutputToken)

	// Convert to micro-dollars
	microDollars := int64(totalCostDollars * 1000000.0)

	return microDollars
}
