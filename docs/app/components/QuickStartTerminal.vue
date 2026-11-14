<script setup lang="ts">
import { ref } from 'vue'

const copied = ref(false)
const command = 'gh extension install hemanshulabs/gh-forge'

const copyCommand = async () => {
  try {
    await navigator.clipboard.writeText(command)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {
    // fallback if clipboard API is restricted
  }
}
</script>

<template>
  <div class="w-full max-w-xl mx-auto my-4 text-left font-mono">
    <div class="rounded-xl border border-slate-800 bg-[#0B0F17] shadow-2xl overflow-hidden ring-1 ring-white/5 transition-all hover:border-slate-700">
      <!-- Terminal Header Bar -->
      <div class="flex items-center justify-between px-4 py-2.5 bg-[#0F172A] border-b border-slate-800">
        <div class="flex items-center gap-2">
          <span class="w-3 h-3 rounded-full bg-[#EF4444] inline-block opacity-80" />
          <span class="w-3 h-3 rounded-full bg-[#F59E0B] inline-block opacity-80" />
          <span class="w-3 h-3 rounded-full bg-[#10B981] inline-block opacity-80" />
          <span class="ml-2 text-xs text-slate-400 font-sans font-medium">terminal</span>
        </div>
        <button
          type="button"
          class="flex items-center gap-1.5 text-xs text-slate-400 hover:text-cyan-400 transition-colors px-2.5 py-1 rounded hover:bg-slate-800/80 cursor-pointer"
          :title="copied ? 'Copied to clipboard!' : 'Copy install command'"
          @click="copyCommand"
        >
          <span v-if="copied" class="text-xs text-cyan-400 font-sans flex items-center gap-1">
            <svg class="w-3.5 h-3.5 text-cyan-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
            Copied!
          </span>
          <span v-else class="text-xs font-sans flex items-center gap-1">
            <svg class="w-3.5 h-3.5 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3" />
            </svg>
            Copy
          </span>
        </button>
      </div>

      <!-- Terminal Body -->
      <div class="p-4 sm:p-5 text-sm leading-relaxed text-slate-300 space-y-3.5 text-left select-text">
        <div>
          <div class="text-xs text-slate-500 mb-1 font-sans"># 1. Install via GitHub CLI</div>
          <div class="flex items-center gap-2 text-left">
            <span class="text-cyan-400 select-none font-bold">$</span>
            <span class="text-slate-100 font-medium">gh extension install hemanshulabs/gh-forge</span>
          </div>
        </div>
        <div class="pt-0.5">
          <div class="text-xs text-slate-500 mb-1 font-sans"># 2. Launch your dashboard</div>
          <div class="flex items-center gap-2 text-left">
            <span class="text-cyan-400 select-none font-bold">$</span>
            <span class="text-amber-300 font-medium">forge</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
