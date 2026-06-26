import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 10 },
    { duration: '20s', target: 10 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export function setup() {
  const signupRes = http.post(`${BASE_URL}/signup`, JSON.stringify({
    name: 'LoadTest User',
    email: `loadtest-${Date.now()}@test.com`,
    password: 'loadtest123',
  }), { headers: { 'Content-Type': 'application/json' } });

  return { email: JSON.parse(signupRes.body).email };
}

export default function (data) {
  const res = http.post(`${BASE_URL}/login`, JSON.stringify({
    email: data.email,
    password: 'loadtest123',
  }), { headers: { 'Content-Type': 'application/json' } });

  check(res, {
    'login status is 200': (r) => r.status === 200,
    'has access_token': (r) => JSON.parse(r.body).access_token !== undefined,
  });

  sleep(0.1);
}
