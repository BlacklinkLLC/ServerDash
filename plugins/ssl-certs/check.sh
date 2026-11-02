#!/bin/sh
# Prints a table of certificate expiry dates for SETTING_HOSTS.
js() { printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | tr -d '\000-\037'; }
warn=${SETTING_WARN_DAYS:-14}
now=$(date +%s)
to_epoch() {
	date -d "$1" +%s 2>/dev/null || date -D '%b %e %H:%M:%S %Y GMT' -d "$1" +%s 2>/dev/null
}
rows="" status="" sep=""
for entry in $(printf '%s\n' "$SETTING_HOSTS" | tr ',' '\n'); do
	host=${entry%%:*}
	port=443
	case "$entry" in *:*) port=${entry##*:} ;; esac
	cert=$(echo | timeout 10 openssl s_client -servername "$host" -connect "$host:$port" 2>/dev/null | openssl x509 -noout -enddate -issuer 2>/dev/null)
	if [ -z "$cert" ]; then
		rows="$rows$sep[\"$(js "$host:$port")\",\"unreachable\",\"\",\"\"]"
		status="$status$sep\"crit\""
		sep=","
		continue
	fi
	end=$(printf '%s\n' "$cert" | sed -n 's/^notAfter=//p')
	issuer=$(printf '%s\n' "$cert" | sed -n 's/^issuer=//p' | sed -n 's/.*O *= *\([^,]*\).*/\1/p')
	exp=$(to_epoch "$end")
	if [ -n "$exp" ]; then
		days=$(( (exp - now) / 86400 ))
		left="$days days"
	else
		days=""
		left="?"
	fi
	st=ok
	if [ -n "$days" ] && [ "$days" -lt 0 ]; then st=crit; left="expired"
	elif [ -n "$days" ] && [ "$days" -lt "$warn" ]; then st=warn; fi
	rows="$rows$sep[\"$(js "$host:$port")\",\"$(js "$end")\",\"$left\",\"$(js "$issuer")\"]"
	status="$status$sep\"$st\""
	sep=","
done
printf '{"columns":["Host","Expires","Left","Issuer"],"rows":[%s],"status":[%s]}\n' "$rows" "$status"
