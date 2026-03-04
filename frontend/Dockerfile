# Frontend image - self-contained multi-stage build
FROM docker.io/library/node:24-alpine AS builder

WORKDIR /app

# Clear proxy env vars inherited from host build environment
ENV HTTP_PROXY="" HTTPS_PROXY="" http_proxy="" https_proxy="" ALL_PROXY="" all_proxy=""

# Install pnpm 9.x to match lockfile format (lockfileVersion: '9.0')
RUN npm install -g pnpm@9 --registry=https://registry.npmmirror.com

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --registry=https://registry.npmmirror.com

COPY . .
RUN pnpm build

FROM docker.io/library/nginx:1.27-alpine

RUN rm /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist/ /usr/share/nginx/html/

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
