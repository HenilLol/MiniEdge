"use client";

import React from 'react';
import { Icon } from '../utils/svgIcons.jsx';

export function ToastContainer({ toasts }) {
  if (!toasts || toasts.length === 0) return null;

  return (
    <div id="global-toast-container" className="toast-container">
      {toasts.map(toast => (
        <div key={toast.id} className={`toast-msg toast-${toast.type}`}>
          <Icon name={toast.type === 'success' ? 'checkCircle' : toast.type === 'danger' ? 'xCircle' : 'alertTriangle'} />
          <span>{toast.message}</span>
        </div>
      ))}
    </div>
  );
}
