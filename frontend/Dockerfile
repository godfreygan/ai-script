# ---------- build stage ----------
FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm config set registry https://registry.npmmirror.com \
    && (npm ci --no-audit --prefer-offline || npm install --no-audit)
COPY . .
RUN npm run build

# ---------- runtime stage ----------
FROM nginx:1.27-alpine

# Create non-root user appuser (uid 1001) belonging to nginx group (gid 101)
RUN adduser -u 1001 -G nginx -s /sbin/nologin -D appuser

# Copy built assets
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Fix permissions: appuser needs read access to html and conf directories
RUN chown -R appuser:nginx /usr/share/nginx/html \
    && chmod -R 755 /usr/share/nginx/html \
    && chown -R appuser:nginx /var/cache/nginx \
    && chown -R appuser:nginx /var/log/nginx \
    && chown -R appuser:nginx /etc/nginx/conf.d \
    && touch /var/run/nginx.pid \
    && chown appuser:nginx /var/run/nginx.pid

EXPOSE 80

USER appuser

CMD ["nginx", "-g", "daemon off;"]
