# frontend/mlops-react-app/Dockerfile
FROM node:18 AS build
WORKDIR /app
# allow passing API base at build time
ARG REACT_APP_API_BASE
ENV REACT_APP_API_BASE=${REACT_APP_API_BASE}
COPY frontend/mlops-react-app/package.json frontend/mlops-react-app/package-lock.json* ./
RUN npm ci --legacy-peer-deps || npm install --legacy-peer-deps
COPY frontend/mlops-react-app ./
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/build /usr/share/nginx/html
EXPOSE 80
CMD ["/usr/sbin/nginx","-g","daemon off;"]
