#!/bin/bash
# Closes the container's network to all but what work on Inkshelf needs: Claude Code's API,
# GitHub, the Go module proxy and npm, plus the hosts in EXTRA_ALLOWED_DOMAINS (a Komga, Mealie
# or Calibre-Web server for make emu). Adapted from Anthropic's reference devcontainer. Run as
# root when the container starts; it can run again, and starts from a clean slate each time.
set -euo pipefail
IFS=$'\n\t'

# Docker's internal DNS rules, kept across the flush.
docker_dns_rules=$(iptables-save -t nat | grep "127\.0\.0\.11" || true)

iptables -F
iptables -X
iptables -t nat -F
iptables -t nat -X
iptables -t mangle -F
iptables -t mangle -X
ipset destroy allowed-domains 2>/dev/null || true

if [ -n "$docker_dns_rules" ]; then
  iptables -t nat -N DOCKER_OUTPUT 2>/dev/null || true
  iptables -t nat -N DOCKER_POSTROUTING 2>/dev/null || true
  echo "$docker_dns_rules" | xargs -L 1 iptables -t nat
fi

# DNS, SSH and localhost before anything else.
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
iptables -A INPUT -p udp --sport 53 -j ACCEPT
iptables -A OUTPUT -p tcp --dport 22 -j ACCEPT
iptables -A INPUT -p tcp --sport 22 -m state --state ESTABLISHED -j ACCEPT
iptables -A INPUT -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT

ipset create allowed-domains hash:net

# GitHub publishes its address ranges.
echo "Fetching GitHub's address ranges"
github_meta=$(curl -sf https://api.github.com/meta)
if [ -z "$github_meta" ] || ! echo "$github_meta" | jq -e '.web and .api and .git' >/dev/null; then
  echo "ERROR: could not fetch GitHub's address ranges" >&2
  exit 1
fi
while read -r cidr; do
  ipset add allowed-domains "$cidr" -exist
done < <(echo "$github_meta" | jq -r '(.web + .api + .git)[] | select(contains(":") | not)' | aggregate -q)

# Every other host by the addresses it resolves to now. Google's (the Go proxy) rotate: should
# go mod download fail later, run sudo /usr/local/bin/init-firewall.sh again.
required_domains=(
  api.anthropic.com
  registry.npmjs.org
  proxy.golang.org
  sum.golang.org
  storage.googleapis.com
)
# Claude Code's login and telemetry: allowed when they resolve, which some no longer do.
optional_domains=(
  claude.ai
  console.anthropic.com
  statsig.anthropic.com
  statsig.com
  sentry.io
)
IFS=' ' read -r -a extra_domains <<< "${EXTRA_ALLOWED_DOMAINS:-}"
optional_domains+=("${extra_domains[@]}")

add_domain() {
  local domain=$1 addresses
  addresses=$(dig +short A "$domain" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' || true)
  if [ -z "$addresses" ]; then
    return 1
  fi
  while read -r address; do
    ipset add allowed-domains "$address" -exist
  done <<< "$addresses"
}

for domain in "${required_domains[@]}"; do
  echo "Allowing $domain"
  if ! add_domain "$domain"; then
    echo "ERROR: $domain does not resolve" >&2
    exit 1
  fi
done
for domain in "${optional_domains[@]}"; do
  # An address, or a name; a host that does not resolve only warns.
  if [[ "$domain" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+(/[0-9]+)?$ ]]; then
    echo "Allowing $domain"
    ipset add allowed-domains "$domain" -exist
  elif add_domain "$domain"; then
    echo "Allowing $domain"
  else
    echo "WARNING: $domain does not resolve, not allowed" >&2
  fi
done

# The host's network, for the ports the editor forwards (the emulator's 8740, the site's 3000).
host_ip=$(ip route | grep default | cut -d" " -f3)
if [ -z "$host_ip" ]; then
  echo "ERROR: no default route to the host" >&2
  exit 1
fi
host_network=$(echo "$host_ip" | sed "s/\.[0-9]*$/.0\/24/")
iptables -A INPUT -s "$host_network" -j ACCEPT
iptables -A OUTPUT -d "$host_network" -j ACCEPT

iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT DROP

iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -m set --match-set allowed-domains dst -j ACCEPT
# Refused at once rather than left to time out.
iptables -A OUTPUT -j REJECT --reject-with icmp-admin-prohibited

echo "Checking the firewall"
if curl --connect-timeout 5 -sf https://example.com >/dev/null 2>&1; then
  echo "ERROR: example.com is reachable" >&2
  exit 1
fi
if ! curl --connect-timeout 5 -sf https://api.github.com/zen >/dev/null 2>&1; then
  echo "ERROR: api.github.com is not reachable" >&2
  exit 1
fi
echo "Firewall up"
