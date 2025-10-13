-- AlterTable
ALTER TABLE "plugins" ADD COLUMN     "shortName" VARCHAR(50);

-- CreateIndex
CREATE INDEX "plugins_shortName_idx" ON "plugins"("shortName");
