import { execSync } from 'node:child_process';
import { randomInt } from 'node:crypto';
import { expect, test } from '@playwright/test';

// Q3-01 + F4-02: login -> create -> list -> download -> delete as a plain User.
// A fresh User per run (a User belongs to at most one Idea), so reruns need no cleanup.

const sha = process.env.GIT_SHA ?? safe(() => execSync('git rev-parse --short HEAD').toString().trim()) ?? 'nosha';
function safe<T>(f: () => T): T | undefined {
  try {
    return f();
  } catch {
    return undefined;
  }
}

const PASSWORD = 'Smk-T3st!pw-Zq7#';
const empNo = String(randomInt(100000, 1000000));
const userName = 'Smoke Tester';
const ideaName = `smoke ${sha} ${Date.now()}`;
const expectedBase = `I_AAID_${userName}_${empNo}_${ideaName}`;
const pdf = Buffer.from('%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n');

test('login → create → list → download → delete', async ({ page, request }) => {
  const dl = await request.get('/api/v1/deadline');
  expect(dl.ok(), `GET /api/v1/deadline answered ${dl.status()}; is the backend up?`).toBe(true);
  const { state } = (await dl.json()) as { state: string };
  expect(state, `Submission deadline state is ${state}, not OPEN: set contest_setting.deadline to a future time (see make smoke).`).toBe('OPEN');

  await page.goto('/register');
  await page.locator('#empNo').fill(empNo);
  await page.locator('#name').fill(userName);
  await page.locator('#email').fill(`smoke${empNo}@example.com`);
  await page.locator('#password').fill(PASSWORD);
  await page.locator('#confirmPassword').fill(PASSWORD);
  await page.getByRole('button', { name: /create account|register/i }).click();

  await expect(page).toHaveURL(/\/login/);
  await page.locator('#empNo').fill(empNo);
  await page.locator('#password').fill(PASSWORD);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page).toHaveURL(/\/ideas$/);

  await page.goto('/ideas/new');
  await page.locator('#category').selectOption('COOL_IDEA');
  await page.locator('#org').selectOption('AAID');
  await page.locator('#direction').selectOption({ index: 1 });
  await page.locator('#name').fill(ideaName);
  await page.locator('#summary').fill('Smoke test summary.');
  await expect(page.getByText(`${expectedBase}.pdf`)).toBeVisible();
  await page.locator('#slides').setInputFiles({ name: `${expectedBase}.pdf`, mimeType: 'application/pdf', buffer: pdf });
  await page.getByRole('button', { name: 'Submit Idea' }).click();

  await expect(page).toHaveURL(/\/ideas\/(?!new$)[^/]+$/);
  await page.goto('/ideas');
  await page.getByRole('link', { name: ideaName }).click();
  await expect(page).toHaveURL(/\/ideas\/(?!new$)[^/]+$/);

  const [file] = await Promise.all([page.waitForEvent('download'), page.getByRole('button', { name: 'Download' }).click()]);
  expect(file.suggestedFilename()).toBe(`${expectedBase}.pdf`);
  const path = await file.path();
  const { readFileSync } = await import('node:fs');
  expect(readFileSync(path).equals(pdf)).toBe(true);

  await page.getByRole('button', { name: 'Delete' }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Delete Idea' }).click();
  await expect(page).toHaveURL(/\/ideas$/);
  await expect(page.getByRole('link', { name: ideaName })).toHaveCount(0);
});
