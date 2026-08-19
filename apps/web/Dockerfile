FROM node:24.13.0-alpine AS dependencies
WORKDIR /workspace
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/package.json
RUN pnpm install --frozen-lockfile

FROM dependencies AS build
COPY apps/web apps/web
RUN pnpm --filter web build

FROM node:24.13.0-alpine AS runtime
ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
ENV PORT=3000
WORKDIR /app
RUN addgroup --system --gid 1001 flightcheck \
    && adduser --system --uid 1001 --ingroup flightcheck flightcheck
COPY --from=build --chown=flightcheck:flightcheck /workspace/apps/web/public ./apps/web/public
COPY --from=build --chown=flightcheck:flightcheck /workspace/apps/web/.next/static ./apps/web/.next/static
COPY --from=build --chown=flightcheck:flightcheck /workspace/apps/web/.next/standalone ./
USER flightcheck
EXPOSE 3000
CMD ["node", "apps/web/server.js"]
