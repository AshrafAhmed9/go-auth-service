import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 5 },
    { duration: '20s', target: 5 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export function setup() {
  const email = `loadtest-refresh-${Date.now()}@test.com`;
  http.post(`${BASE_URL}/signup`, JSON.stringify({
    name: 'LoadTest User',
    email: email,
    password: 'loadtest123',
  }), { headers: { 'Content-Type': 'application/json' } });

  const loginRes = http.post(`${BASE_URL}/login`, JSON.stringify({
    email: email,
    password: 'loadtest123',
  }), { headers: { 'Content-Type': 'application/json' } });

  return {
    email: email,
    password: 'loadtest123',
  };
}

export default function (data) {
  const loginRes = http.post(`${BASE_URL}/login`, JSON.stringify({
    email: data.email,
    password: data.password,
  }), { headers: { 'Content-Type': 'application/json' } });

  if (loginRes.status !== 200) {
    return;
  }

  const refreshToken = JSON.parse(loginRes.body).refresh_token;

  const res = http.post(`${BASE_URL}/refresh`, JSON.stringify({
    refresh_token: refreshToken,
  }), { headers: { 'Content-Type': 'application/json' } });

  check(res, {
    'refresh status is 200': (r) => r.status === 200,
    'has new access_token': (r) => JSON.parse(r.body).access_token !== undefined,
  });

  sleep(0.5);
}
