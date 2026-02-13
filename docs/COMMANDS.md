# FlashDB Commands Reference

## String Commands

### SET key value
Set key to hold the string value. If key already holds a value, it is overwritten.

**Time complexity:** O(1)

**Return value:** Simple string reply: OK

**Example:**
```
SET mykey "Hello"
```

---

### GET key
Get the value of key. If the key does not exist, nil is returned.

**Time complexity:** O(1)

**Return value:** Bulk string reply: the value of key, or nil when key does not exist.

**Example:**
```
GET mykey
```

---

### DEL key [key ...]
Removes the specified keys. A key is ignored if it does not exist.

**Time complexity:** O(N) where N is the number of keys

**Return value:** Integer reply: The number of keys that were removed.

**Example:**
```
DEL key1 key2 key3
```

---

## Key Commands

### EXISTS key [key ...]
Returns if key exists.

**Time complexity:** O(N) where N is the number of keys

**Return value:** Integer reply: The number of keys that exist.

**Example:**
```
EXISTS key1 key2
```

---

### KEYS pattern
Returns all keys matching pattern. Only `*` pattern is supported (returns all keys).

**Time complexity:** O(N)

**Return value:** Array reply: list of keys matching pattern.

**Example:**
```
KEYS *
```

---

## Server Commands

### PING
Returns PONG. This command is useful for testing connection.

**Time complexity:** O(1)

**Return value:** Simple string reply: PONG

**Example:**
```
PING
```

---

### DBSIZE
Return the number of keys in the currently-selected database.

**Time complexity:** O(1)

**Return value:** Integer reply: number of keys

**Example:**
```
DBSIZE
```

---

### FLUSHDB
Delete all the keys of the currently selected DB. This command never fails.

**Time complexity:** O(N)

**Return value:** Simple string reply: OK

**Example:**
```
FLUSHDB
```

---

## Security & Operations Commands

### AUTH password
### AUTH username password
Authenticate to the server. In legacy mode, a single password is checked. In ACL mode, a username and password pair is validated against the configured users.

**Time complexity:** O(N) where N is the number of configured ACL users

**Return value:** Simple string reply: OK on success, error on failure.

**Example:**
```
AUTH mysecretpassword
AUTH admin mysecretpassword
```

---

### ACL WHOAMI
Return the username of the currently authenticated user.

**Time complexity:** O(1)

**Return value:** Bulk string reply: the username

**Example:**
```
ACL WHOAMI
```

---

### ACL LIST
List all configured ACL users with their permissions.

**Time complexity:** O(N) where N is the number of users

**Return value:** Array reply: list of user permission strings

**Example:**
```
ACL LIST
```

---

### SLOWLOG GET [count]
Return the last *count* entries from the slow query log (default: 10). Entries are returned newest-first.

**Time complexity:** O(N) where N is the number of entries returned

**Return value:** Array reply: slow log entries

**Example:**
```
SLOWLOG GET
SLOWLOG GET 25
```

---

### SLOWLOG LEN
Return the number of entries in the slow query log.

**Time complexity:** O(1)

**Return value:** Integer reply: number of slow log entries

**Example:**
```
SLOWLOG LEN
```

---

### SLOWLOG RESET
Clear the slow query log.

**Time complexity:** O(1)

**Return value:** Simple string reply: OK

**Example:**
```
SLOWLOG RESET
```

---

## Sorted Set Commands

### ZADD key score member [score member ...]
Adds all the specified members with the specified scores to the sorted set.

**Time complexity:** O(log(N)) for each item added

**Return value:** Integer reply: number of elements added

**Example:**
```
ZADD myzset 1 "one" 2 "two" 3 "three"
```

---

### ZSCORE key member
Returns the score of member in the sorted set at key.

**Time complexity:** O(1)

**Return value:** Bulk string reply: the score of member

**Example:**
```
ZSCORE myzset "one"
```

---

### ZRANK key member
Returns the rank of member in the sorted set at key, with scores ordered low to high.

**Time complexity:** O(log(N))

**Return value:** Integer reply: the rank of member

**Example:**
```
ZRANK myzset "one"
```

---

### ZREVRANK key member
Returns the rank of member in the sorted set at key, with scores ordered high to low.

**Time complexity:** O(log(N))

**Return value:** Integer reply: the rank of member

**Example:**
```
ZREVRANK myzset "three"
```

---

### ZRANGE key start stop [WITHSCORES]
Returns the specified range of elements in the sorted set at key.

**Time complexity:** O(log(N)+M)

**Return value:** Array reply: list of elements in the specified range

**Example:**
```
ZRANGE myzset 0 -1 WITHSCORES
```

---

### ZREVRANGE key start stop [WITHSCORES]
Returns the specified range of elements in reverse order.

**Time complexity:** O(log(N)+M)

**Return value:** Array reply: list of elements in the specified range

**Example:**
```
ZREVRANGE myzset 0 -1 WITHSCORES
```

---

### ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]
Returns all elements in the sorted set at key with a score between min and max.

**Time complexity:** O(log(N)+M)

**Return value:** Array reply: list of elements in the specified score range

**Example:**
```
ZRANGEBYSCORE myzset -inf +inf
```

---

### ZREM key member [member ...]
Removes the specified members from the sorted set.

**Time complexity:** O(M*log(N))

**Return value:** Integer reply: number of members removed

**Example:**
```
ZREM myzset "one"
```

---

### ZINCRBY key increment member
Increments the score of member in the sorted set by increment.

**Time complexity:** O(log(N))

**Return value:** Bulk string reply: the new score of member

**Example:**
```
ZINCRBY myzset 2 "one"
```

---

### ZCARD key
Returns the cardinality (number of elements) of the sorted set.

**Time complexity:** O(1)

**Return value:** Integer reply: the cardinality of the sorted set

**Example:**
```
ZCARD myzset
```

---

### ZCOUNT key min max
Returns the number of elements in the sorted set with a score between min and max.

**Time complexity:** O(log(N))

**Return value:** Integer reply: the number of elements in the specified score range

**Example:**
```
ZCOUNT myzset 1 3
```

---

### ZPOPMIN key [count]
Removes and returns the members with the lowest scores.

**Time complexity:** O(log(N)*M)

**Return value:** Array reply: list of popped elements and scores

**Example:**
```
ZPOPMIN myzset 2
```

---

### ZPOPMAX key [count]
Removes and returns the members with the highest scores.

**Time complexity:** O(log(N)*M)

**Return value:** Array reply: list of popped elements and scores

**Example:**
```
ZPOPMAX myzset 2
```

---

## Transaction Commands

### MULTI
Marks the start of a transaction block.

**Return value:** Simple string reply: OK

**Example:**
```
MULTI
SET key1 "value1"
SET key2 "value2"
EXEC
```

---

### EXEC
Executes all commands issued after MULTI.

**Return value:** Array reply: results of each command

---

### DISCARD
Flushes all previously queued commands and restores connection state to normal.

**Return value:** Simple string reply: OK

---

## Pub/Sub Commands

### SUBSCRIBE channel [channel ...]
Subscribes the client to the specified channels.

**Example:**
```
SUBSCRIBE news updates
```

---

### UNSUBSCRIBE [channel ...]
Unsubscribes the client from the given channels.

**Example:**
```
UNSUBSCRIBE news
```

---

### PUBLISH channel message
Posts a message to the given channel.

**Return value:** Integer reply: number of clients that received the message

**Example:**
```
PUBLISH news "Hello World"
```

---

## Authentication Commands

### AUTH password
Authenticates to the server.

**Return value:** Simple string reply: OK on success, Error on failure

**Example:**
```
AUTH mypassword
```

---

## Hash Commands

### HSET key field value [field value ...]
Sets the specified fields to their respective values in the hash stored at key. This command overwrites the values of specified fields that exist in the hash. If key does not exist, a new key holding a hash is created.

**Time complexity:** O(N) where N is the number of field/value pairs

**Return value:** Integer reply: the number of fields that were added (not updated).

**Example:**
```
HSET user:1 name "Alice" age "30"
```

---

### HGET key field
Returns the value associated with field in the hash stored at key.

**Time complexity:** O(1)

**Return value:** Bulk string reply: the value associated with field, or nil when field is not present.

**Example:**
```
HGET user:1 name
```

---

### HMSET key field value [field value ...]
Sets the specified fields to their respective values in the hash stored at key. Like HSET but returns OK instead of count.

**Time complexity:** O(N)

**Return value:** Simple string reply: OK

---

### HMGET key field [field ...]
Returns the values associated with the specified fields in the hash stored at key.

**Time complexity:** O(N)

**Return value:** Array reply: list of values associated with the given fields, in the same order as they are requested.

---

### HDEL key field [field ...]
Removes the specified fields from the hash stored at key.

**Time complexity:** O(N) where N is the number of fields to be removed

**Return value:** Integer reply: the number of fields that were removed from the hash.

**Example:**
```
HDEL user:1 age
```

---

### HEXISTS key field
Returns if field is an existing field in the hash stored at key.

**Time complexity:** O(1)

**Return value:** Integer reply: 1 if the field exists, 0 if it does not or key does not exist.

---

### HLEN key
Returns the number of fields contained in the hash stored at key.

**Time complexity:** O(1)

**Return value:** Integer reply: number of fields in the hash, or 0 when key does not exist.

---

### HGETALL key
Returns all fields and values of the hash stored at key.

**Time complexity:** O(N) where N is the size of the hash

**Return value:** Array reply: list of fields and their values stored in the hash, or an empty list when key does not exist.

**Example:**
```
HGETALL user:1
```

---

### HKEYS key
Returns all field names in the hash stored at key.

**Time complexity:** O(N)

**Return value:** Array reply: list of fields in the hash.

---

### HVALS key
Returns all values in the hash stored at key.

**Time complexity:** O(N)

**Return value:** Array reply: list of values in the hash.

---

### HINCRBY key field increment
Increments the number stored at field in the hash stored at key by increment. If key or field does not exist, the value is set to 0 before performing the operation.

**Time complexity:** O(1)

**Return value:** Integer reply: the value of key after the increment.

**Example:**
```
HINCRBY user:1 age 1
```

---

### HINCRBYFLOAT key field increment
Increment the specified field of a hash by the given floating-point value.

**Time complexity:** O(1)

**Return value:** Bulk string reply: the value of the field after the increment.

---

### HSETNX key field value
Sets field in the hash stored at key to value, only if field does not yet exist.

**Time complexity:** O(1)

**Return value:** Integer reply: 1 if the field was set, 0 if it already existed.

---

## List Commands

### LPUSH key element [element ...]
Insert all the specified values at the head of the list stored at key. If key does not exist, it is created as empty list before performing the push operations.

**Time complexity:** O(N) where N is the number of elements

**Return value:** Integer reply: the length of the list after the push operations.

**Example:**
```
LPUSH mylist "world" "hello"
```

---

### RPUSH key element [element ...]
Insert all the specified values at the tail of the list stored at key.

**Time complexity:** O(N)

**Return value:** Integer reply: the length of the list after the push operations.

**Example:**
```
RPUSH mylist "hello" "world"
```

---

### LPOP key
Removes and returns the first element of the list stored at key.

**Time complexity:** O(1)

**Return value:** Bulk string reply: the value of the first element, or nil when key does not exist.

---

### RPOP key
Removes and returns the last element of the list stored at key.

**Time complexity:** O(1)

**Return value:** Bulk string reply: the value of the last element, or nil when key does not exist.

---

### LLEN key
Returns the length of the list stored at key.

**Time complexity:** O(1)

**Return value:** Integer reply: the length of the list at key.

---

### LINDEX key index
Returns the element at index in the list stored at key. Negative indices count from the end (-1 is the last element).

**Time complexity:** O(N) where N is the number of elements to traverse

**Return value:** Bulk string reply: the requested element, or nil when index is out of range.

**Example:**
```
LINDEX mylist 0
```

---

### LSET key index element
Sets the list element at index to element.

**Time complexity:** O(N)

**Return value:** Simple string reply: OK, or error if index is out of range.

---

### LRANGE key start stop
Returns the specified elements of the list stored at key. The offsets start and stop are zero-based indices. Negative indices count from the end.

**Time complexity:** O(S+N)

**Return value:** Array reply: list of elements in the specified range.

**Example:**
```
LRANGE mylist 0 -1
```

---

### LINSERT key BEFORE|AFTER pivot element
Inserts element in the list stored at key either before or after the reference value pivot.

**Time complexity:** O(N)

**Return value:** Integer reply: the length of the list after the insert operation, or -1 when the pivot is not found.

**Example:**
```
LINSERT mylist BEFORE "World" "There"
```

---

### LREM key count element
Removes the first count occurrences of elements equal to element from the list stored at key. count > 0: head to tail, count < 0: tail to head, count = 0: remove all.

**Time complexity:** O(N+S)

**Return value:** Integer reply: the number of removed elements.

---

### LTRIM key start stop
Trim an existing list so that it will contain only the specified range of elements.

**Time complexity:** O(N)

**Return value:** Simple string reply: OK

**Example:**
```
LTRIM mylist 1 -1
```

---

## Set Commands

### SADD key member [member ...]
Add the specified members to the set stored at key. If key does not exist, a new set is created.

**Time complexity:** O(N) where N is the number of members

**Return value:** Integer reply: the number of elements that were added to the set, not including already existing members.

**Example:**
```
SADD myset "Hello" "World"
```

---

### SREM key member [member ...]
Remove the specified members from the set stored at key.

**Time complexity:** O(N)

**Return value:** Integer reply: the number of members that were removed from the set.

---

### SISMEMBER key member
Returns if member is a member of the set stored at key.

**Time complexity:** O(1)

**Return value:** Integer reply: 1 if the element is a member of the set, 0 otherwise.

---

### SCARD key
Returns the set cardinality (number of elements) of the set stored at key.

**Time complexity:** O(1)

**Return value:** Integer reply: the cardinality of the set.

---

### SMEMBERS key
Returns all the members of the set value stored at key.

**Time complexity:** O(N)

**Return value:** Array reply: all elements of the set.

**Example:**
```
SMEMBERS myset
```

---

### SRANDMEMBER key [count]
When called with just the key argument, returns a random element from the set. When called with count, returns an array of count distinct elements.

**Time complexity:** O(N) where N is the absolute value of count

**Return value:** Without count: Bulk string reply. With count: Array reply.

---

### SPOP key [count]
Removes and returns one or more random members from the set stored at key.

**Time complexity:** O(N) where N is the value of count

**Return value:** Without count: Bulk string reply. With count: Array reply.

---

### SINTER key [key ...]
Returns the members of the set resulting from the intersection of all the given sets.

**Time complexity:** O(N*M) worst case

**Return value:** Array reply: list of members in the intersection.

**Example:**
```
SINTER set1 set2
```

---

### SUNION key [key ...]
Returns the members of the set resulting from the union of all the given sets.

**Time complexity:** O(N)

**Return value:** Array reply: list of members in the union.

---

### SDIFF key [key ...]
Returns the members of the set resulting from the difference between the first set and all the successive sets.

**Time complexity:** O(N)

**Return value:** Array reply: list of members in the difference.

---

## Time Series Commands

### TS.ADD key timestamp value
Add a data point to a time series. If timestamp is `*`, the current server time is used.

**Time complexity:** O(log N) where N is the number of data points in the series

**Return value:** Integer reply: the timestamp of the added data point.

**Example:**
```
TS.ADD sensor:temp * 23.5
TS.ADD sensor:temp 1700000000 22.1
```

---

### TS.GET key
Get the latest data point from a time series.

**Time complexity:** O(1)

**Return value:** Array reply: [timestamp, value], or nil if the series does not exist.

**Example:**
```
TS.GET sensor:temp
```

---

### TS.RANGE key start end
Get data points within a time range (inclusive). Use `0` and `+inf` for the full range.

**Time complexity:** O(log N + M) where M is the number of returned points

**Return value:** Array reply: list of [timestamp, value] pairs.

**Example:**
```
TS.RANGE sensor:temp 1700000000 1700003600
TS.RANGE sensor:temp 0 +inf
```

---

### TS.INFO key
Get metadata about a time series including count, min, max, avg values, and time range.

**Time complexity:** O(1)

**Return value:** Array reply: key-value pairs with series metadata.

**Example:**
```
TS.INFO sensor:temp
```

---

### TS.DEL key
Delete an entire time series and all its data points.

**Time complexity:** O(1)

**Return value:** Simple string reply: OK, or error if the series does not exist.

**Example:**
```
TS.DEL sensor:temp
```

---

### TS.KEYS
List all time series keys.

**Time complexity:** O(N) where N is the number of time series

**Return value:** Array reply: list of time series key names.

**Example:**
```
TS.KEYS
```

---

## Hot Key Detection Commands

### HOTKEYS [count]
Return the most frequently accessed keys. Default count is 10.

**Time complexity:** O(N log K) where N is the number of tracked keys and K is count

**Return value:** Array reply: list of [key, access_count] pairs, sorted by frequency descending.

**Example:**
```
HOTKEYS
HOTKEYS 20
```

---

## Snapshot Commands

### SNAPSHOT CREATE [name]
Create a point-in-time snapshot of the entire database. An optional name can be provided.

**Time complexity:** O(N) where N is the number of keys

**Return value:** Simple string reply: the snapshot ID.

**Example:**
```
SNAPSHOT CREATE
SNAPSHOT CREATE before-migration
```

---

### SNAPSHOT LIST
List all available snapshots.

**Time complexity:** O(S) where S is the number of snapshots

**Return value:** Array reply: list of snapshot metadata (ID, name, timestamp, key count).

**Example:**
```
SNAPSHOT LIST
```

---

### SNAPSHOT RESTORE id
Restore the database from a snapshot, replacing all current data.

**Time complexity:** O(N) where N is the number of keys in the snapshot

**Return value:** Simple string reply: OK.

**Example:**
```
SNAPSHOT RESTORE 20260101-150405
```

---

### SNAPSHOT DELETE id
Delete a snapshot.

**Time complexity:** O(1)

**Return value:** Simple string reply: OK, or error if the snapshot does not exist.

**Example:**
```
SNAPSHOT DELETE 20260101-150405
```

---

## Change Data Capture (CDC) Commands

### CDC LATEST [count]
Get the most recent CDC events. Default count is 50.

**Time complexity:** O(K) where K is count

**Return value:** Array reply: list of CDC event records (seq, op, key, value, timestamp).

**Example:**
```
CDC LATEST
CDC LATEST 100
```

---

### CDC SINCE id
Get all CDC events with a sequence number greater than the given ID.

**Time complexity:** O(K) where K is the number of matching events

**Return value:** Array reply: list of CDC event records.

**Example:**
```
CDC SINCE 42
```

---

### CDC STATS
Get CDC stream statistics including total events, buffer utilization, and subscriber count.

**Time complexity:** O(1)

**Return value:** Array reply: key-value pairs with stream statistics.

**Example:**
```
CDC STATS
```

---

## Benchmark Commands

### BENCHMARK [operations]
Run a built-in benchmark with the specified number of SET/GET operations. Default is 1000, maximum is 100000.

**Time complexity:** O(N) where N is the number of operations

**Return value:** Array reply: benchmark results including ops/sec, average latency, p50/p99 latencies, duration, and error count.

**Example:**
```
BENCHMARK
BENCHMARK 10000
```
