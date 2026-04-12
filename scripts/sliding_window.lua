-- KEYS[1] = rate limit key (e.g., "rl:ip:1.2.3.4")
-- ARGV[1] = window size in microseconds
-- ARGV[2] = max requests allowed in the window
-- ARGV[3] = unique request id
--
-- Returns: {allowed, remaining, reset_at_unix, retry_after_seconds}

local key = KEYS[1]
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local request_id = ARGV[3]

local time = redis.call('TIME')
local now_us = tonumber(time[1]) * 1000000 + tonumber(time[2])
local window_start = now_us - window

redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

local count = redis.call('ZCARD', key)

local reset_at = math.ceil((now_us + window) / 1000000)

if count < limit then
    redis.call('ZADD', key, now_us, now_us .. ':' .. request_id)
    redis.call('PEXPIRE', key, math.ceil(window / 1000) + 1000)
    return {1, limit - count - 1, reset_at, 0}
else
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local retry_after = 0
    if #oldest > 0 then
        local oldest_score = tonumber(oldest[2])
        retry_after = math.ceil((oldest_score + window - now_us) / 1000000)
        if retry_after < 0 then
            retry_after = 0
        end
    end
    return {0, 0, reset_at, retry_after}
end
