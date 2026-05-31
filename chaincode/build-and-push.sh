#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_IMAGE_NAME="vote-ledger-chaincode"

die() {
    echo "Error: $*" >&2
    exit 1
}

get_dockerhub_username() {
    local username
    username="$(docker info --format '{{.Username}}' 2>/dev/null || true)"
    if [[ -n "$username" && "$username" != "<no value>" ]]; then
        echo "$username"
        return 0
    fi

    local docker_config="${DOCKER_CONFIG:-$HOME/.docker}"
    local config_file="${docker_config}/config.json"
    [[ -f "$config_file" ]] || return 1

    local auth
    auth="$(
        sed -nE '/(https:\/\/index\.docker\.io\/v1\/|index\.docker\.io|registry-1\.docker\.io)/,/}/ s/.*"auth"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' "$config_file" \
            | head -n 1
    )"
    if [[ -n "$auth" ]]; then
        username="$(printf '%s' "$auth" | base64 -d 2>/dev/null | sed 's/:.*//')"
        if [[ -n "$username" ]]; then
            echo "$username"
            return 0
        fi
    fi

    return 1
}

command -v docker >/dev/null 2>&1 || die "docker is not installed or not in PATH"
docker info >/dev/null 2>&1 || die "docker daemon is not running, or current user cannot access Docker"

DETECTED_DOCKERHUB_USERNAME="$(get_dockerhub_username || true)"
[[ -n "$DETECTED_DOCKERHUB_USERNAME" ]] || die "Docker Hub login is required. Run: docker login"
DOCKERHUB_USERNAME="${DOCKERHUB_USERNAME:-$DETECTED_DOCKERHUB_USERNAME}"

if [[ -z "${IMAGE_NAME:-}" ]]; then
    read -r -p "Enter image name [${DEFAULT_IMAGE_NAME}]: " IMAGE_NAME
    IMAGE_NAME="${IMAGE_NAME:-$DEFAULT_IMAGE_NAME}"
fi

read -r -p "Enter Docker image version (for example: 1.0.0): " VERSION

[[ -n "$IMAGE_NAME" ]] || die "image name must not be empty"
[[ -n "$VERSION" ]] || die "Docker image version must not be empty"

if [[ ! "$VERSION" =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]]; then
    die "invalid version tag. Use only letters, numbers, underscores, dots, or dashes; max length is 128 characters"
fi

IMAGE="${DOCKERHUB_USERNAME}/${IMAGE_NAME}:${VERSION}"

echo "Building image: ${IMAGE}"
docker build -t "$IMAGE" "$SCRIPT_DIR"

echo "Pushing image: ${IMAGE}"
docker push "$IMAGE"

echo "Done: ${IMAGE}"
