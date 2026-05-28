#!/usr/bin/env bash
# watch-http-log.sh [PATH_TO_LOGFILE]

pending=""

tail -f "${1:-logfile.jsonl}" | while IFS= read -r line; do
  msg=$(echo "$line" | jq -r '.msg // empty' 2>/dev/null)

  case "$msg" in
    "http request")
      pending="$line"
      ;;
    "http response")
      if [[ -n "$pending" ]]; then
        req_url=$(echo "$pending" | jq -r '.url')
        req_method=$(echo "$pending" | jq -r '.method')
        req_body=$(echo "$pending" | jq -r '.body' | jq '.' 2>/dev/null || echo "(no body)")

        res_status=$(echo "$line" | jq -r '.status')
        res_ms=$(echo "$line" | jq -r '.duration_ms')
        res_body=$(echo "$line" | jq -r '.body' | jq '.' 2>/dev/null || echo "(no body)")

        printf "\n\033[1;33m▶ REQUEST  %s %s\033[0m\n" "$req_method" "$req_url"
        echo "$req_body"
        printf "\n\033[1;32m◀ RESPONSE %s %sms\033[0m\n" "$res_status" "$res_ms"
        echo "$res_body"
        printf '%0.s─' {1..60}; echo
        pending=""
      fi
      ;;
  esac
done
