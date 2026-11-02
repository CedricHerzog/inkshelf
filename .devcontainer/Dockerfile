# A container for working on Inkshelf, with Claude Code and Spec Kit: everything make verify,
# make icons and make docs need. Its network is the host's, open: the reader under remote
# control, the servers of make emu and the module proxies are all reached from it.

ARG UV_VERSION=0.13.1

# uv as its authors advise, copied from their image, to install Spec Kit's CLI.
FROM ghcr.io/astral-sh/uv:${UV_VERSION} AS uv

# Go as in go.mod.
FROM golang:1.27.2-trixie

ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=1000
ARG NODE_MAJOR=24
ARG GOLANGCI_LINT_VERSION=v2.14.0
ARG CLAUDE_CODE_VERSION=latest
ARG SPEC_KIT_VERSION=1.2.5

# The icons' tools (rsvg-convert, ImageMagick), Spec Kit's Python, network tools to look into a
# connection to the reader, and a shell to work in.
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl dnsutils fzf gh git gnupg imagemagick iproute2 iputils-ping \
    jq less librsvg2-bin make man-db procps python3 unzip zsh \
  && rm -rf /var/lib/apt/lists/*

# The Makefile calls ImageMagick 7's magick. Should the distribution's package only ship
# convert, it takes the same arguments.
RUN if ! command -v magick >/dev/null; then \
      printf '#!/bin/sh\nexec convert "$@"\n' > /usr/local/bin/magick && chmod +x /usr/local/bin/magick; \
    fi

# Node, for the learning site (docs) and for Claude Code.
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_MAJOR}.x | bash - \
  && apt-get install -y --no-install-recommends nodejs \
  && rm -rf /var/lib/apt/lists/*

# golangci-lint as a release binary, as its authors advise, at the version .golangci.yml was
# written for.
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
  | sh -s -- -b /usr/local/bin ${GOLANGCI_LINT_VERSION}

RUN groupadd --gid ${USER_GID} ${USERNAME} \
  && useradd --uid ${USER_UID} --gid ${USER_GID} --shell /bin/zsh --create-home ${USERNAME}

COPY --from=uv /uv /uvx /usr/local/bin/

# Global npm packages in the user's home, so that Claude Code can update itself.
ENV NPM_CONFIG_PREFIX=/home/${USERNAME}/.npm-global
ENV PATH=/home/${USERNAME}/.npm-global/bin:${PATH}

# The directories the named volumes are mounted on exist, owned by the user, so that the volumes
# start out the user's too.
RUN mkdir -p /go/pkg/mod /home/${USERNAME}/.cache/go-build /home/${USERNAME}/.claude \
    /home/${USERNAME}/.npm-global /commandhistory /workspace \
  && touch /commandhistory/.zsh_history \
  && chown -R ${USERNAME}:${USERNAME} /go /home/${USERNAME} /commandhistory /workspace

# The bind-mounted repository can show up as root's, which git then refuses (and with it go
# build's VCS stamping).
RUN git config --system --add safe.directory /workspace

USER ${USERNAME}

RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}

# Spec Kit's specify CLI, at a pinned version: to upgrade it, change SPEC_KIT_VERSION and
# rebuild. uv uses the system's Python rather than
# downloading its own.
ENV UV_PYTHON_PREFERENCE=only-system
ENV UV_TOOL_BIN_DIR=/home/${USERNAME}/.local/bin
ENV PATH=/home/${USERNAME}/.local/bin:${PATH}
RUN uv tool install specify-cli==${SPEC_KIT_VERSION}

RUN printf '%s\n' \
    'HISTFILE=/commandhistory/.zsh_history' 'HISTSIZE=10000' 'SAVEHIST=10000' \
    'setopt inc_append_history share_history' \
    'autoload -Uz compinit && compinit' \
    '[ -f /usr/share/doc/fzf/examples/key-bindings.zsh ] && source /usr/share/doc/fzf/examples/key-bindings.zsh' \
    'PROMPT="%F{cyan}%~%f %# "' \
    > /home/${USERNAME}/.zshrc

ENV DEVCONTAINER=true
ENV SHELL=/bin/zsh
ENV GOCACHE=/home/${USERNAME}/.cache/go-build

WORKDIR /workspace
