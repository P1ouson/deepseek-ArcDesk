import { useUser } from '../hooks/useUser';
import React from 'react';

export function App() {
  return React.createElement('div', null, useUser());
}
