# Leaderboard Case Study

#### Requirements 
* Go Language
* Persistence (for board/user metadata, such as Postgres)
* Docker
* Git

You need to develop a REST API which is used to manage a leaderboard system. It has operations for managing boards and tracking user scores. 
Each leaderboard can have an optional schedule that, once triggered, 
resets and clears all user scores.

The solution should be pushed to a public git repository and you should share the link to the repository with us.

---

## Create Board

Creates a new leaderboard. If a schedule is provided, the leaderboard resets at the specified interval, clearing all scores.

`POST /boards`
##### Sample Request
```json
{
  "name": "Weekly Tournament",
  "description": "Global leaderboard for weekly tournament",
  "schedule": {
    "type": "interval",
    "intervalSeconds": 604800
  }
}
```

##### Responses

###### 201 Created
```json
{
  "boardId": "board_123",
  "name": "Weekly Tournament",
  "description": "Global leaderboard for weekly tournament",
  "schedule": {
    "type": "interval",
    "intervalSeconds": 604800
  }
}
```

###### 400 Bad Request
```json
{
  "error": "Invalid board name"
}
```

---

## List Boards
Lists all available leaderboards with basic metadata.

`GET /boards`

##### Responses

###### 200 Success
```json
[
  {
    "boardId": "board_123",
    "name": "Weekly Tournament"
  },
  {
    "boardId": "board_456",
    "name": "All-time Top Scores"
  }
]
```

---

## Get Board

Returns details for a specific leaderboard, including its current reset schedule and the timestamp for the next scheduled reset (if applicable).

`GET /boards/{boardId}`

##### Responses

###### 200 Success
```json
{
  "boardId": "board_123",
  "name": "Weekly Tournament",
  "description": "Global leaderboard for weekly tournament",
  "createdAt": "2026-01-01T12:00:00Z",
  "schedule": {
    "type": "interval",
    "intervalSeconds": 604800
  },
  "nextResetAt": "2026-01-08T12:00:00Z"
}
```

###### 404 Not Found
```json
{
  "error": "Board not found"
}
```

---

## Set Score

Creates or updates a user's score on the specified leaderboard. 
- **Ranking**: Higher scores rank higher (Descending order).
- **Update Logic**: Each call overwrites the previous score for that user (it is not incremental).
- **Reset Schedule**: If the board has a schedule, the score is stored within the current active period. When a reset occurs, all existing scores are permanently deleted.
- **Tie-breaking**: In case of a tie, the user who achieved the score first should be ranked higher.

`POST /boards/{boardId}/scores`

```json
{
  "userId": "user_789",
  "score": 1500
}
```

##### Responses

###### 200 Success
```json
{
  "boardId": "board_123",
  "userId": "user_789",
  "score": 1500
}
```

###### 404 Not Found
```json
{
  "error": "Board not found"
}
```

---

## Get Top Scores

Retrieves the top `n` users ranked by score. If no users have participated yet, an empty list is returned.

`GET /boards/{boardId}/scores?n=10`

##### Responses

###### 200 Success
```json
[
  {
    "userId": "user_1",
    "score": 5000
  },
  {
    "userId": "user_789",
    "score": 1500
  }
]
```

###### 400 Bad Request
```json
{
  "error": "Invalid value for n"
}
```

###### 404 Not Found
```json
{
  "error": "Board not found"
}
```

---

## Get Score Surroundings (Optional)

Retrieves the score for a specific user along with the `n` users immediately above them and `n` users immediately below them in the leaderboard rankings.

`GET /boards/{boardId}/scores/{userId}/surroundings?n=5`

##### Responses

###### 200 Success
```json
{
  "user": {
    "userId": "user_789",
    "score": 1500
  },
  "above": [
    {
      "userId": "user_above_1",
      "score": 1510
    }
  ],
  "below": [
    {
      "userId": "user_below_1",
      "score": 1490
    }
  ]
}
```

###### 404 Not Found
```json
{
  "error": "Board or user not found"
}
```

--- 

### Expectations

* This service should be written in Go lang, you can use any framework or library you need. 
* All leaderboard data should be persistent.
* Use appropriate data structures and algorithms for leaderboard efficiency (e.g., Indexes). 
* Dockerfile and docker-compose should be provided in the project.
* Write tests.

#### Extras (Optional)

* Add an endpoint to populate a board with `n` mock users and random scores to facilitate testing.
* Get Score Surroundings endpoint is optional.

