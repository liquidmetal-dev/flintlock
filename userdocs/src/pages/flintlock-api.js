import React from 'react';
import { RedocStandalone } from 'redoc';

export default function Hello() {
  return (
    <RedocStandalone specUrl="https://raw.githubusercontent.com/liquidmetal-dev/flintlock/main/api/services/microvm/v1alpha1/microvms.swagger.json"/>
  )
}
