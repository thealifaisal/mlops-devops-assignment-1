# frontend/mlops-react-app/Dockerfile
FROM node:18 AS build
WORKDIR /app
COPY frontend/mlops-react-app/package.json frontend/mlops-react-app/package-lock.json* ./
RUN npm ci || npm install
COPY frontend/mlops-react-app ./
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/build /usr/share/nginx/html
EXPOSE 80
CMD ["/usr/sbin/nginx","-g","daemon off;"]
