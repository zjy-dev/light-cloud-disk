# Frontend image with self-contained multi-stage build
FROM docker.io/library/node:24-alpine AS builder

WORKDIR /app

# Clear proxy env vars inherited from host build environment
ENV HTTP_PROXY="" HTTPS_PROXY="" http_proxy="" https_proxy="" ALL_PROXY="" all_proxy=""

# Install pnpm via corepack (ships with Node 24)
RUN corepack enable && corepack prepare pnpm@10 --activate

COPY package.json pnpm-lock.yaml .npmrc ./
RUN pnpm install --frozen-lockfile --registry=https://registry.npmmirror.com

COPY . .
RUN pnpm build

FROM docker.io/library/nginx:1.27-alpine

RUN rm /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist/ /usr/share/nginx/html/

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
