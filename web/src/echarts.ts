/**
 * ECharts module registration.
 *
 * `echarts/core` is tree-shakeable — only the renderer, chart types,
 * and components imported here land in the bundle. Imported once from
 * main.ts so `vue-echarts`' <VChart> resolves everywhere.
 *
 * Used by: GradientStudio (line — perceptual-lightness ramp) and
 * ContrastChecker's "Suggest fixes" (bar — contrast-vs-lightness).
 */
import { use } from 'echarts/core';
import { CanvasRenderer } from 'echarts/renderers';
import { BarChart, LineChart } from 'echarts/charts';
import {
  GridComponent,
  MarkLineComponent,
  TooltipComponent,
} from 'echarts/components';

use([
  CanvasRenderer,
  LineChart,
  BarChart,
  GridComponent,
  TooltipComponent,
  MarkLineComponent,
]);
