import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 10 },
    { duration: '20s', target: 10 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export function setup() {
  const email = `loadtest-profile-${Date.now()}@test.com`;
  http.post(`${BASE_URL}/signup`, JSON.stringify({
    name: 'LoadTest User',
    email: email,
    password: 'loadtest123',
  }), { headers: { 'Content-Type': 'application/json' } });

  const loginRes = http.post(`${BASE_URL}/login`, JSON.stringify({
    email: email,
    password: 'loadtest123',
  }), { headers: { 'Content-Type': 'application/json' } });

  return { token: JSON.parse(loginRes.body).access_token };
}

export default function (data) {
  const res = http.get(`${BASE_URL}/profile`, {
    headers: {
      'Authorization': `Bearer ${data.token}`,
      'Content-Type': 'application/json',
    },
  });

  check(res, {
    'profile status is 200': (r) => r.status === 200,
  });

  sleep(0.1);
}
