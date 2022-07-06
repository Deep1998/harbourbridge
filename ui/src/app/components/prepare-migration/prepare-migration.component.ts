import { Component, OnInit } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { TargetDetailsFormComponent } from '../target-details-form/target-details-form.component';

@Component({
  selector: 'app-prepare-migration',
  templateUrl: './prepare-migration.component.html',
  styleUrls: ['./prepare-migration.component.scss']
})
export class PrepareMigrationComponent implements OnInit {

  displayedColumns = [
    'Title',
    'Source',
    'Destination',
  ]
  constructor(private dialog: MatDialog) { }

  ngOnInit(): void {
  }
  openTargetDetailsForm() {
    let openDialog = this.dialog.open(TargetDetailsFormComponent, {
      width: '30vw',
      minWidth: '400px',
      maxWidth: '500px',
    })
    openDialog.afterClosed().subscribe(() => {
      
    })
  }

}
