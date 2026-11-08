import type { FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { Field } from '@/shared/ui/field';
import { Button } from '@/shared/ui/button';
import { PageHeading } from '../layouts';

const noSubmit = (e: FormEvent) => e.preventDefault();

export function Login() {
  const { t } = useTranslation();
  return (
    <section className="mx-auto max-w-md">
      <PageHeading title={t('nav.signIn')} />
      <form onSubmit={noSubmit}>
        <Field label={t('fields.empNo')}>{(p) => <input {...p} name="empNo" autoComplete="username" required />}</Field>
        <Field label={t('fields.password')}>
          {(p) => <input {...p} name="password" type="password" autoComplete="current-password" required />}
        </Field>
        <Button type="submit">{t('nav.signIn')}</Button>
      </form>
      <p className="mt-4 text-sm">
        {t('pages.login.noAccount')}{' '}
        <Link to="/register" className="underline">
          {t('nav.register')}
        </Link>
      </p>
    </section>
  );
}

export function Register() {
  const { t } = useTranslation();
  return (
    <section className="mx-auto max-w-md">
      <PageHeading title={t('nav.register')} />
      <form onSubmit={noSubmit}>
        <Field label={t('fields.empNo')}>{(p) => <input {...p} name="empNo" autoComplete="username" required />}</Field>
        <Field label={t('fields.name')}>{(p) => <input {...p} name="name" autoComplete="name" required />}</Field>
        <Field label={t('fields.email')}>{(p) => <input {...p} name="email" type="email" autoComplete="email" required />}</Field>
        <Field label={t('fields.password')} hint={t('pages.register.passwordHint')}>
          {(p) => <input {...p} name="password" type="password" autoComplete="new-password" required />}
        </Field>
        <Field label={t('fields.confirmPassword')}>
          {(p) => <input {...p} name="confirmPassword" type="password" autoComplete="new-password" required />}
        </Field>
        <Button type="submit">{t('nav.register')}</Button>
      </form>
      <p className="mt-4 text-sm">
        {t('pages.register.haveAccount')}{' '}
        <Link to="/login" className="underline">
          {t('nav.signIn')}
        </Link>
      </p>
    </section>
  );
}
