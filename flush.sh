#!/bin/bash

BASE_URL="http://localhost:8080/api/v1/collections/users/documents"
N=10000

for ((i=1; i<=N; i++)); do
  # Generate random name/email/city/age
  name="User_$RANDOM"
  email="user${RANDOM}@example.com"
  age=$((18 + RANDOM % 50))
  cities=("Boston" "New York" "London" "Berlin" "Tokyo" "San Francisco")
  city=${cities[$RANDOM % ${#cities[@]}]}

  data=$(cat <<EOF
{
    "name": "$name",
    "email": "$email",
    "age": $age,
    "city": "$city"
}
EOF
)

  # Send POST request
  curl -s -X POST "$BASE_URL" \
    -H "Content-Type: application/json" \
    -d "$data" > /dev/null

  # Optional: show progress every 100 requests
  if (( i % 100 == 0 )); then
    echo "Inserted $i documents..."
  fi
done

echo "✅ Finished inserting $N user documents."
