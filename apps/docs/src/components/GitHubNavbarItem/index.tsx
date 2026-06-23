import Link from '@docusaurus/Link';
import clsx from 'clsx';
import type {Props} from '@theme/NavbarItem/DefaultNavbarItem';
import type {ReactNode} from 'react';

export default function GitHubNavbarItem({
  mobile,
  className,
  href,
  label,
  position: _position,
  ...props
}: Props): ReactNode {
  const text = typeof label === 'string' ? label : 'GitHub';

  return (
    <Link
      {...props}
      className={clsx(mobile ? 'menu__link' : 'navbar__item navbar__link', className)}
      href={href}
      rel="noopener noreferrer"
      target="_blank"
    >
      {text}
    </Link>
  );
}
