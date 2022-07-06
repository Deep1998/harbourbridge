import { Component, Inject, OnInit } from '@angular/core';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { MatDialogRef, MAT_DIALOG_DATA } from '@angular/material/dialog';
import ITargetDetails from 'src/app/model/target-details';

@Component({
  selector: 'app-target-details-form',
  templateUrl: './target-details-form.component.html',
  styleUrls: ['./target-details-form.component.scss']
})
export class TargetDetailsFormComponent implements OnInit {
targetDetailsForm : FormGroup
  fetch: any;
  snack: any;
  constructor(
    private fb: FormBuilder,
    @Inject(MAT_DIALOG_DATA) public data: ITargetDetails,
  private dialogRef: MatDialogRef<TargetDetailsFormComponent>) {
    this.targetDetailsForm = this.fb.group({
      TargetDB: ['', Validators.required],
    })
    dialogRef.disableClose = true
   }

  ngOnInit(): void {
  }

  updateTargetDetails() {
    let formValue = this.targetDetailsForm.value
    let payload: ITargetDetails = {
      TargetDB: formValue.TargetDB,
    }

    this.fetch.setSpannerConfig(payload).subscribe({
      next: (res: TargetDetailsFormComponent) => {
        this.snack.openSnackBar('Target details updated successfully', 'Close', 5)
        this.dialogRef.close({ ...res })
      },
      error: (err: any) => {
        this.snack.openSnackBar(err.message, 'Close')
      },
    })
  }
}
