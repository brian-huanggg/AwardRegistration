import { useEffect, useRef, type FormEvent } from 'react';
import { Leaf } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { Button } from '@/shared/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card';
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/shared/ui/field';
import { Input } from '@/shared/ui/input';

const noSubmit = (e: FormEvent) => e.preventDefault();

/** One h1 per page; focus moves to it after a route change and the title updates (Frontend.md §8). */
function useAuthHeading(title: string) {
  const { t } = useTranslation();
  const ref = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    document.title = `${title} · ${t('app.name')}`;
    ref.current?.focus();
  }, [title, t]);
  return ref;
}

/** shadcn `login-02`: two columns, form on the left, brand panel on the right (hidden below lg). */
export function Login() {
  const { t } = useTranslation();
  const heading = useAuthHeading(t('nav.signIn'));
  return (
    <div className="grid overflow-hidden rounded-xl border lg:grid-cols-2">
      <div className="flex items-center justify-center p-6 md:p-10">
        <form className="flex w-full max-w-xs flex-col gap-6" onSubmit={noSubmit}>
          <FieldGroup>
            <div className="flex flex-col items-center gap-1 text-center">
              <h1 ref={heading} tabIndex={-1} className="text-2xl font-bold outline-none">
                {t('pages.login.title')}
              </h1>
              <p className="text-sm text-balance text-muted-foreground">{t('pages.login.description')}</p>
            </div>
            <Field>
              <FieldLabel htmlFor="empNo">{t('fields.empNo')}</FieldLabel>
              <Input id="empNo" name="empNo" autoComplete="username" required />
            </Field>
            <Field>
              <FieldLabel htmlFor="password">{t('fields.password')}</FieldLabel>
              <Input id="password" name="password" type="password" autoComplete="current-password" required />
            </Field>
            <Field>
              <Button type="submit">{t('nav.signIn')}</Button>
              <FieldDescription className="text-center">
                {t('pages.login.noAccount')}{' '}
                <Link to="/register" className="underline underline-offset-4">
                  {t('nav.register')}
                </Link>
              </FieldDescription>
            </Field>
          </FieldGroup>
        </form>
      </div>
      <div
        aria-hidden="true"
        className="hidden flex-col items-center justify-center gap-3 bg-muted p-10 text-center lg:flex"
      >
        <div className="flex size-12 items-center justify-center rounded-md bg-primary text-primary-foreground">
          <Leaf className="size-6" />
        </div>
        <p className="text-xl font-semibold">{t('pages.login.panelTitle')}</p>
        <p className="max-w-xs text-sm text-balance text-muted-foreground">{t('pages.login.panelBody')}</p>
      </div>
    </div>
  );
}

/** shadcn `signup-01`: a single centred card. */
export function Register() {
  const { t } = useTranslation();
  const heading = useAuthHeading(t('nav.register'));
  return (
    <div className="mx-auto w-full max-w-sm">
      <Card>
        <CardHeader>
          <CardTitle>
            <h1 ref={heading} tabIndex={-1} className="outline-none">
              {t('pages.register.title')}
            </h1>
          </CardTitle>
          <CardDescription>{t('pages.register.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={noSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="empNo">{t('fields.empNo')}</FieldLabel>
                <Input id="empNo" name="empNo" autoComplete="username" required />
              </Field>
              <Field>
                <FieldLabel htmlFor="name">{t('fields.name')}</FieldLabel>
                <Input id="name" name="name" autoComplete="name" required />
              </Field>
              <Field>
                <FieldLabel htmlFor="email">{t('fields.email')}</FieldLabel>
                <Input id="email" name="email" type="email" autoComplete="email" required />
              </Field>
              <Field>
                <FieldLabel htmlFor="password">{t('fields.password')}</FieldLabel>
                <Input id="password" name="password" type="password" autoComplete="new-password" required />
                <FieldDescription>{t('pages.register.passwordHint')}</FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="confirmPassword">{t('fields.confirmPassword')}</FieldLabel>
                <Input id="confirmPassword" name="confirmPassword" type="password" autoComplete="new-password" required />
              </Field>
              <Field>
                <Button type="submit">{t('nav.register')}</Button>
                <FieldDescription className="text-center">
                  {t('pages.register.haveAccount')}{' '}
                  <Link to="/login" className="underline underline-offset-4">
                    {t('nav.signIn')}
                  </Link>
                </FieldDescription>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
